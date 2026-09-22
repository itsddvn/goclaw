//go:build sqlite || sqliteonly

package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// SQLiteWorkstationContactGrantStore implements Contact-specific workstation grants.
type SQLiteWorkstationContactGrantStore struct {
	db *sql.DB
}

func NewSQLiteWorkstationContactGrantStore(db *sql.DB) *SQLiteWorkstationContactGrantStore {
	return &SQLiteWorkstationContactGrantStore{db: db}
}

func (s *SQLiteWorkstationContactGrantStore) Grant(ctx context.Context, grant *store.WorkstationContactGrant) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	grant.TenantID = tid
	grant.CreatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.QueryRowContext(ctx,
		`SELECT cc.channel_type, cc.sender_id, cc.display_name, cc.username
		 FROM channel_contacts cc CROSS JOIN workstations w
		 WHERE cc.id = ? AND cc.tenant_id = ? AND cc.contact_type = 'user'
		   AND w.id = ? AND w.tenant_id = ?`,
		grant.ContactID.String(), tid.String(), grant.WorkstationID.String(), tid.String(),
	).Scan(&grant.ChannelType, &grant.SenderID, &grant.DisplayName, &grant.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	var persistedCreatedAt sqliteTime
	err = tx.QueryRowContext(ctx,
		`INSERT INTO workstation_contact_grants
		 (workstation_id, contact_id, tenant_id, created_by, created_at)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(workstation_id, contact_id) DO UPDATE SET
		 created_by = excluded.created_by
		 WHERE workstation_contact_grants.tenant_id = excluded.tenant_id
		 RETURNING created_at`,
		grant.WorkstationID.String(), grant.ContactID.String(), tid.String(),
		grant.CreatedBy, grant.CreatedAt.Format(time.RFC3339Nano),
	).Scan(&persistedCreatedAt)
	if err != nil {
		return err
	}
	grant.CreatedAt = persistedCreatedAt.Time
	return tx.Commit()
}

func (s *SQLiteWorkstationContactGrantStore) Revoke(ctx context.Context, workstationID, contactID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM workstation_contact_grants
		 WHERE workstation_id = ? AND contact_id = ? AND tenant_id = ?`,
		workstationID.String(), contactID.String(), tid.String(),
	)
	return err
}

func (s *SQLiteWorkstationContactGrantStore) HasAccess(ctx context.Context, workstationID, contactID uuid.UUID) (bool, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil || contactID == uuid.Nil {
		return false, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM workstation_contact_grants g
		 JOIN channel_contacts cc ON cc.id = g.contact_id AND cc.tenant_id = g.tenant_id
		 JOIN workstations w ON w.id = g.workstation_id AND w.tenant_id = g.tenant_id
		 WHERE g.workstation_id = ? AND g.contact_id = ?
		   AND g.tenant_id = ? AND cc.contact_type = 'user'`,
		workstationID.String(), contactID.String(), tid.String(),
	).Scan(&count)
	return count == 1, err
}

func (s *SQLiteWorkstationContactGrantStore) ListForWorkstation(ctx context.Context, workstationID uuid.UUID) ([]store.WorkstationContactGrant, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.workstation_id, g.contact_id, g.tenant_id,
		        cc.channel_type, cc.sender_id, cc.display_name, cc.username,
		        g.created_by, g.created_at
		 FROM workstation_contact_grants g
		 JOIN channel_contacts cc ON cc.id = g.contact_id AND cc.tenant_id = g.tenant_id
		 JOIN workstations w ON w.id = g.workstation_id AND w.tenant_id = g.tenant_id
		 WHERE g.workstation_id = ? AND g.tenant_id = ? AND cc.contact_type = 'user'
		 ORDER BY COALESCE(cc.display_name, cc.username, cc.sender_id), cc.channel_type, cc.sender_id`,
		workstationID.String(), tid.String(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]store.WorkstationContactGrant, 0)
	for rows.Next() {
		var grant store.WorkstationContactGrant
		var workstationIDText, contactIDText, tenantIDText string
		var createdAt sqliteTime
		if err := rows.Scan(
			&workstationIDText, &contactIDText, &tenantIDText,
			&grant.ChannelType, &grant.SenderID, &grant.DisplayName, &grant.Username,
			&grant.CreatedBy, &createdAt,
		); err != nil {
			return nil, err
		}
		grant.WorkstationID, _ = uuid.Parse(workstationIDText)
		grant.ContactID, _ = uuid.Parse(contactIDText)
		grant.TenantID, _ = uuid.Parse(tenantIDText)
		grant.CreatedAt = createdAt.Time
		result = append(result, grant)
	}
	return result, rows.Err()
}
