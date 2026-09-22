//go:build sqlite || sqliteonly

package sqlitestore

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func seedWorkstationLinkTestWorkstation(t *testing.T, db *sql.DB, tenantID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	workstationID := uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO workstations
		 (id, workstation_key, tenant_id, name, backend_type, metadata, default_env, active)
		 VALUES (?,?,?,?,?,?,?,1)`,
		workstationID.String(), key, tenantID.String(), key, "ssh", []byte("{}"), []byte("{}"),
	); err != nil {
		t.Fatalf("seed workstation: %v", err)
	}
	return workstationID
}

func TestSQLiteAgentWorkstationLinkStoreEnforcesTenantAndDefault(t *testing.T) {
	db := newHookTestDB(t)
	tenantID, agentID := seedHookTenantAgent(t, db)
	otherTenantID, otherAgentID := seedHookTenantAgent(t, db)
	firstID := seedWorkstationLinkTestWorkstation(t, db, tenantID, "first")
	secondID := seedWorkstationLinkTestWorkstation(t, db, tenantID, "second")
	otherID := seedWorkstationLinkTestWorkstation(t, db, otherTenantID, "other")
	linkStore := NewSQLiteAgentWorkstationLinkStore(db)
	ctx := sqliteTenantCtx(tenantID)

	first := &store.AgentWorkstationLink{AgentID: agentID, WorkstationID: firstID, IsDefault: true}
	if err := linkStore.Link(ctx, first); err != nil {
		t.Fatalf("Link first: %v", err)
	}
	if allowed, err := linkStore.HasAccess(ctx, agentID, firstID); err != nil || !allowed {
		t.Fatalf("HasAccess first = %v, %v; want true", allowed, err)
	}

	second := &store.AgentWorkstationLink{AgentID: agentID, WorkstationID: secondID, IsDefault: true}
	if err := linkStore.Link(ctx, second); err != nil {
		t.Fatalf("Link second: %v", err)
	}
	links, err := linkStore.ListForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("ListForAgent: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2", len(links))
	}
	defaults := 0
	for _, link := range links {
		if link.IsDefault {
			defaults++
			if link.WorkstationID != secondID {
				t.Fatalf("default workstation = %s, want %s", link.WorkstationID, secondID)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("default links = %d, want 1", defaults)
	}

	if err := linkStore.Link(ctx, &store.AgentWorkstationLink{AgentID: agentID, WorkstationID: otherID}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-tenant workstation Link error = %v, want sql.ErrNoRows", err)
	}
	if err := linkStore.Link(ctx, &store.AgentWorkstationLink{AgentID: otherAgentID, WorkstationID: firstID}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-tenant agent Link error = %v, want sql.ErrNoRows", err)
	}
	if _, err := db.Exec(
		`INSERT INTO agent_workstation_links (agent_id, workstation_id, tenant_id, is_default)
		 VALUES (?,?,?,0)`,
		otherAgentID.String(), firstID.String(), tenantID.String(),
	); err != nil {
		t.Fatalf("seed legacy mismatched link: %v", err)
	}
	workstationLinks, err := linkStore.ListForWorkstation(ctx, firstID)
	if err != nil {
		t.Fatalf("ListForWorkstation: %v", err)
	}
	for _, link := range workstationLinks {
		if link.AgentID == otherAgentID {
			t.Fatal("ListForWorkstation exposed a legacy cross-tenant link")
		}
	}
	if allowed, err := linkStore.HasAccess(sqliteTenantCtx(otherTenantID), agentID, secondID); err != nil || allowed {
		t.Fatalf("cross-tenant HasAccess = %v, %v; want false", allowed, err)
	}
	if err := linkStore.Unlink(ctx, agentID, secondID); err != nil {
		t.Fatalf("Unlink second: %v", err)
	}
	if allowed, err := linkStore.HasAccess(ctx, agentID, secondID); err != nil || allowed {
		t.Fatalf("HasAccess after revoke = %v, %v; want false", allowed, err)
	}
}

func TestSQLiteAgentWorkstationLinkStoreSetDefaultRequiresExistingGrant(t *testing.T) {
	db := newHookTestDB(t)
	tenantID, agentID := seedHookTenantAgent(t, db)
	workstationID := seedWorkstationLinkTestWorkstation(t, db, tenantID, "unassigned")
	linkStore := NewSQLiteAgentWorkstationLinkStore(db)

	err := linkStore.SetDefault(sqliteTenantCtx(tenantID), agentID, workstationID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetDefault error = %v, want sql.ErrNoRows", err)
	}
}
