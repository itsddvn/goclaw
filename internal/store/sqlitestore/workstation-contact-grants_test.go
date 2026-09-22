//go:build sqlite || sqliteonly

package sqlitestore

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func seedWorkstationGrantTestContact(t *testing.T, db *sql.DB, tenantID uuid.UUID, channelType, senderID, displayName, contactType string) uuid.UUID {
	t.Helper()
	contactID := uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO channel_contacts (id, channel_type, sender_id, display_name, contact_type, tenant_id)
		 VALUES (?,?,?,?,?,?)`,
		contactID.String(), channelType, senderID, displayName, contactType, tenantID.String(),
	); err != nil {
		t.Fatalf("seed Contact: %v", err)
	}
	return contactID
}

func seedWorkstationGrantTestTenantUser(t *testing.T, db *sql.DB, tenantID uuid.UUID, userID string) uuid.UUID {
	t.Helper()
	tenantUserID := uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO tenant_users (id, tenant_id, user_id, role) VALUES (?,?,?,'member')`,
		tenantUserID.String(), tenantID.String(), userID,
	); err != nil {
		t.Fatalf("seed tenant user: %v", err)
	}
	return tenantUserID
}

func TestSQLiteWorkstationContactGrantStoreEnforcesExactTenantContact(t *testing.T) {
	db := newHookTestDB(t)
	tenantA, _ := seedHookTenantAgent(t, db)
	tenantB, _ := seedHookTenantAgent(t, db)
	workstationA := seedWorkstationLinkTestWorkstation(t, db, tenantA, "contact-grant-a")
	workstationB := seedWorkstationLinkTestWorkstation(t, db, tenantB, "contact-grant-b")
	contactA := seedWorkstationGrantTestContact(t, db, tenantA, "telegram", "alice", "Alice", "user")
	contactA2 := seedWorkstationGrantTestContact(t, db, tenantA, "discord", "alice-discord", "Alice", "user")
	contactB := seedWorkstationGrantTestContact(t, db, tenantB, "telegram", "bob", "Bob", "user")
	groupA := seedWorkstationGrantTestContact(t, db, tenantA, "telegram", "group-1", "Group", "group")
	mergedUserID := seedWorkstationGrantTestTenantUser(t, db, tenantA, "alice-merged")
	if _, err := db.Exec(`UPDATE channel_contacts SET merged_id = ? WHERE id IN (?,?)`, mergedUserID.String(), contactA.String(), contactA2.String()); err != nil {
		t.Fatalf("merge test Contacts: %v", err)
	}
	grants := NewSQLiteWorkstationContactGrantStore(db)
	ctxA := sqliteTenantCtx(tenantA)

	grant := &store.WorkstationContactGrant{
		WorkstationID: workstationA,
		ContactID:     contactA,
		CreatedBy:     "admin",
	}
	if err := grants.Grant(ctxA, grant); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if grant.TenantID != tenantA || grant.SenderID != "alice" || grant.DisplayName == nil || *grant.DisplayName != "Alice" {
		t.Fatalf("populated grant = %#v", grant)
	}
	if allowed, err := grants.HasAccess(ctxA, workstationA, contactA); err != nil || !allowed {
		t.Fatalf("HasAccess Contact A = %v, %v; want true", allowed, err)
	}
	if allowed, err := grants.HasAccess(sqliteTenantCtx(tenantB), workstationA, contactA); err != nil || allowed {
		t.Fatalf("cross-tenant HasAccess = %v, %v; want false", allowed, err)
	}
	if allowed, err := grants.HasAccess(ctxA, workstationA, contactB); err != nil || allowed {
		t.Fatalf("different Contact HasAccess = %v, %v; want false", allowed, err)
	}
	if allowed, err := grants.HasAccess(ctxA, workstationA, contactA2); err != nil || allowed {
		t.Fatalf("merged sibling Contact HasAccess = %v, %v; want false", allowed, err)
	}
	contactStore := NewSQLiteContactStore(db)
	if resolved, err := contactStore.ResolveContactID(ctxA, "telegram", "alice"); err != nil || resolved != contactA {
		t.Fatalf("ResolveContactID = %s, %v; want %s", resolved, err, contactA)
	}

	if err := grants.Grant(ctxA, &store.WorkstationContactGrant{WorkstationID: workstationB, ContactID: contactA}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-tenant workstation Grant error = %v, want sql.ErrNoRows", err)
	}
	if err := grants.Grant(ctxA, &store.WorkstationContactGrant{WorkstationID: workstationA, ContactID: contactB}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-tenant Contact Grant error = %v, want sql.ErrNoRows", err)
	}
	if err := grants.Grant(ctxA, &store.WorkstationContactGrant{WorkstationID: workstationA, ContactID: groupA}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("group Contact Grant error = %v, want sql.ErrNoRows", err)
	}

	listed, err := grants.ListForWorkstation(ctxA, workstationA)
	if err != nil || len(listed) != 1 || listed[0].ContactID != contactA {
		t.Fatalf("ListForWorkstation = %#v, %v", listed, err)
	}
	if err := grants.Revoke(ctxA, workstationA, contactA); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if allowed, err := grants.HasAccess(ctxA, workstationA, contactA); err != nil || allowed {
		t.Fatalf("HasAccess after revoke = %v, %v; want false", allowed, err)
	}
}

func TestEnsureSchemaMigratesWorkstationContactGrantsFromV60(t *testing.T) {
	db := openTestDBAtVersion(t, 60)
	if _, err := db.Exec(`DROP TABLE workstation_contact_grants`); err != nil {
		t.Fatalf("drop latest-only table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS workstation_user_grants (
		workstation_id TEXT NOT NULL,
		tenant_user_id TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		created_by TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (workstation_id, tenant_user_id)
	)`); err != nil {
		t.Fatalf("restore v60 table: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema v60 to current: %v", err)
	}
	var tableName string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'workstation_contact_grants'`,
	).Scan(&tableName); err != nil {
		t.Fatalf("workstation_contact_grants not created: %v", err)
	}
	if tableName != "workstation_contact_grants" {
		t.Fatalf("table name = %q", tableName)
	}
	var legacyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'workstation_user_grants'`).Scan(&legacyCount); err != nil {
		t.Fatalf("lookup legacy table: %v", err)
	}
	if legacyCount != 0 {
		t.Fatal("legacy workstation_user_grants table still exists")
	}
}
