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

// PGAgentWorkstationLinkStore implements store.AgentWorkstationLinkStore backed by PostgreSQL.
type PGAgentWorkstationLinkStore struct {
	db *sql.DB
}

// NewPGAgentWorkstationLinkStore creates a PGAgentWorkstationLinkStore.
func NewPGAgentWorkstationLinkStore(db *sql.DB) *PGAgentWorkstationLinkStore {
	return &PGAgentWorkstationLinkStore{db: db}
}

func (s *PGAgentWorkstationLinkStore) Link(ctx context.Context, link *store.AgentWorkstationLink) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	link.TenantID = tid
	link.CreatedAt = time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var entityExists int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1
		 FROM agents a CROSS JOIN workstations w
		 WHERE a.id = $1 AND a.tenant_id = $3
		   AND w.id = $2 AND w.tenant_id = $3
		 FOR UPDATE OF a, w`,
		link.AgentID, link.WorkstationID, tid,
	).Scan(&entityExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	if link.IsDefault {
		if _, err := tx.ExecContext(ctx,
			`UPDATE agent_workstation_links SET is_default = FALSE
			 WHERE agent_id = $1 AND tenant_id = $2`,
			link.AgentID, tid,
		); err != nil {
			return err
		}
	}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO agent_workstation_links (agent_id, workstation_id, tenant_id, is_default, created_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (agent_id, workstation_id) DO UPDATE
		 SET is_default = EXCLUDED.is_default
		 WHERE agent_workstation_links.tenant_id = EXCLUDED.tenant_id
		 RETURNING created_at`,
		link.AgentID, link.WorkstationID, tid, link.IsDefault, link.CreatedAt,
	).Scan(&link.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PGAgentWorkstationLinkStore) Unlink(ctx context.Context, agentID, workstationID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_workstation_links WHERE agent_id = $1 AND workstation_id = $2 AND tenant_id = $3`,
		agentID, workstationID, tid,
	)
	return err
}

func (s *PGAgentWorkstationLinkStore) HasAccess(ctx context.Context, agentID, workstationID uuid.UUID) (bool, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return false, nil
	}
	var allowed bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM agent_workstation_links l
			JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
			JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
			WHERE l.agent_id = $1 AND l.workstation_id = $2 AND l.tenant_id = $3
		)`,
		agentID, workstationID, tid,
	).Scan(&allowed)
	return allowed, err
}

func (s *PGAgentWorkstationLinkStore) SetDefault(ctx context.Context, agentID, workstationID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var linkExists int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1
		 FROM agents a
		 JOIN agent_workstation_links l ON l.agent_id = a.id AND l.tenant_id = a.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.agent_id = $1 AND l.workstation_id = $2 AND l.tenant_id = $3
		 FOR UPDATE OF a`,
		agentID, workstationID, tid,
	).Scan(&linkExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	// Clear previous default for this agent.
	if _, err := tx.ExecContext(ctx,
		`UPDATE agent_workstation_links SET is_default = FALSE
		 WHERE agent_id = $1 AND tenant_id = $2`,
		agentID, tid,
	); err != nil {
		return err
	}
	// Set new default.
	if _, err := tx.ExecContext(ctx,
		`UPDATE agent_workstation_links SET is_default = TRUE
		 WHERE agent_id = $1 AND workstation_id = $2 AND tenant_id = $3`,
		agentID, workstationID, tid,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PGAgentWorkstationLinkStore) ListForAgent(ctx context.Context, agentID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.agent_id, l.workstation_id, l.tenant_id, l.is_default, l.created_at
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.agent_id = $1 AND l.tenant_id = $2`,
		agentID, tid,
	)
	if err != nil {
		return nil, err
	}
	return scanLinks(rows)
}

func (s *PGAgentWorkstationLinkStore) ListForWorkstation(ctx context.Context, workstationID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.agent_id, l.workstation_id, l.tenant_id, l.is_default, l.created_at
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.workstation_id = $1 AND l.tenant_id = $2`,
		workstationID, tid,
	)
	if err != nil {
		return nil, err
	}
	return scanLinks(rows)
}

func scanLinks(rows *sql.Rows) ([]store.AgentWorkstationLink, error) {
	defer rows.Close()
	var result []store.AgentWorkstationLink
	for rows.Next() {
		var l store.AgentWorkstationLink
		if err := rows.Scan(&l.AgentID, &l.WorkstationID, &l.TenantID, &l.IsDefault, &l.CreatedAt); err != nil {
			continue
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
