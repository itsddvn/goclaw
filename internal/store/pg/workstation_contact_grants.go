package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGWorkstationContactGrantStore implements Contact-specific workstation grants.
type PGWorkstationContactGrantStore struct {
	db *sql.DB
}

func NewPGWorkstationContactGrantStore(db *sql.DB) *PGWorkstationContactGrantStore {
	return &PGWorkstationContactGrantStore{db: db}
}

func (s *PGWorkstationContactGrantStore) Grant(ctx context.Context, grant *store.WorkstationContactGrant) error {
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
		 WHERE cc.id = $1 AND cc.tenant_id = $3 AND cc.contact_type = 'user'
		   AND w.id = $2 AND w.tenant_id = $3
		 FOR UPDATE OF cc, w`,
		grant.ContactID, grant.WorkstationID, tid,
	).Scan(&grant.ChannelType, &grant.SenderID, &grant.DisplayName, &grant.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO workstation_contact_grants
		 (workstation_id, contact_id, tenant_id, created_by, created_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (workstation_id, contact_id) DO UPDATE
		 SET created_by = EXCLUDED.created_by
		 WHERE workstation_contact_grants.tenant_id = EXCLUDED.tenant_id
		 RETURNING created_at`,
		grant.WorkstationID, grant.ContactID, tid, grant.CreatedBy, grant.CreatedAt,
	).Scan(&grant.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PGWorkstationContactGrantStore) Revoke(ctx context.Context, workstationID, contactID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM workstation_contact_grants
		 WHERE workstation_id = $1 AND contact_id = $2 AND tenant_id = $3`,
		workstationID, contactID, tid,
	)
	return err
}

func (s *PGWorkstationContactGrantStore) HasAccess(ctx context.Context, workstationID, contactID uuid.UUID) (bool, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil || contactID == uuid.Nil {
		return false, nil
	}
	var allowed bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM workstation_contact_grants g
			JOIN channel_contacts cc ON cc.id = g.contact_id AND cc.tenant_id = g.tenant_id
			JOIN workstations w ON w.id = g.workstation_id AND w.tenant_id = g.tenant_id
			WHERE g.workstation_id = $1 AND g.contact_id = $2
			  AND g.tenant_id = $3 AND cc.contact_type = 'user'
		)`,
		workstationID, contactID, tid,
	).Scan(&allowed)
	return allowed, err
}

func (s *PGWorkstationContactGrantStore) ListForWorkstation(ctx context.Context, workstationID uuid.UUID) ([]store.WorkstationContactGrant, error) {
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
		 WHERE g.workstation_id = $1 AND g.tenant_id = $2 AND cc.contact_type = 'user'
		 ORDER BY COALESCE(cc.display_name, cc.username, cc.sender_id), cc.channel_type, cc.sender_id`,
		workstationID, tid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]store.WorkstationContactGrant, 0)
	for rows.Next() {
		var grant store.WorkstationContactGrant
		if err := rows.Scan(
			&grant.WorkstationID, &grant.ContactID, &grant.TenantID,
			&grant.ChannelType, &grant.SenderID, &grant.DisplayName, &grant.Username,
			&grant.CreatedBy, &grant.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, grant)
	}
	return result, rows.Err()
}
