//go:build sqlite || sqliteonly

package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// SQLiteAgentWorkstationLinkStore implements store.AgentWorkstationLinkStore backed by SQLite.
type SQLiteAgentWorkstationLinkStore struct {
	db *sql.DB
}

// NewSQLiteAgentWorkstationLinkStore creates a SQLiteAgentWorkstationLinkStore.
func NewSQLiteAgentWorkstationLinkStore(db *sql.DB) *SQLiteAgentWorkstationLinkStore {
	return &SQLiteAgentWorkstationLinkStore{db: db}
}

func (s *SQLiteAgentWorkstationLinkStore) Link(ctx context.Context, link *store.AgentWorkstationLink) error {
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

	var entityCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM agents a CROSS JOIN workstations w
		 WHERE a.id = ? AND a.tenant_id = ?
		   AND w.id = ? AND w.tenant_id = ?`,
		link.AgentID.String(), tid.String(), link.WorkstationID.String(), tid.String(),
	).Scan(&entityCount); err != nil {
		return err
	}
	if entityCount != 1 {
		return sql.ErrNoRows
	}
	if link.IsDefault {
		if _, err := tx.ExecContext(ctx,
			`UPDATE agent_workstation_links SET is_default = 0 WHERE agent_id = ? AND tenant_id = ?`,
			link.AgentID.String(), tid.String(),
		); err != nil {
			return err
		}
	}
	var persistedCreatedAt sqliteTime
	err = tx.QueryRowContext(ctx,
		`INSERT INTO agent_workstation_links
		 (agent_id, workstation_id, tenant_id, is_default, created_at)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(agent_id, workstation_id) DO UPDATE SET
		 is_default = excluded.is_default
		 WHERE agent_workstation_links.tenant_id = excluded.tenant_id
		 RETURNING created_at`,
		link.AgentID.String(), link.WorkstationID.String(), tid.String(),
		boolToInt(link.IsDefault), link.CreatedAt.Format(time.RFC3339Nano),
	).Scan(&persistedCreatedAt)
	if err != nil {
		return err
	}
	link.CreatedAt = persistedCreatedAt.Time
	return tx.Commit()
}

func (s *SQLiteAgentWorkstationLinkStore) Unlink(ctx context.Context, agentID, workstationID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_workstation_links WHERE agent_id = ? AND workstation_id = ? AND tenant_id = ?`,
		agentID.String(), workstationID.String(), tid.String(),
	)
	return err
}

func (s *SQLiteAgentWorkstationLinkStore) HasAccess(ctx context.Context, agentID, workstationID uuid.UUID) (bool, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return false, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.agent_id = ? AND l.workstation_id = ? AND l.tenant_id = ?`,
		agentID.String(), workstationID.String(), tid.String(),
	).Scan(&count)
	return count == 1, err
}

func (s *SQLiteAgentWorkstationLinkStore) SetDefault(ctx context.Context, agentID, workstationID uuid.UUID) error {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant_id required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var linkCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.agent_id = ? AND l.workstation_id = ? AND l.tenant_id = ?`,
		agentID.String(), workstationID.String(), tid.String(),
	).Scan(&linkCount); err != nil {
		return err
	}
	if linkCount != 1 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE agent_workstation_links SET is_default = 0 WHERE agent_id = ? AND tenant_id = ?`,
		agentID.String(), tid.String(),
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE agent_workstation_links SET is_default = 1
		 WHERE agent_id = ? AND workstation_id = ? AND tenant_id = ?`,
		agentID.String(), workstationID.String(), tid.String(),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteAgentWorkstationLinkStore) ListForAgent(ctx context.Context, agentID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.agent_id, l.workstation_id, l.tenant_id, l.is_default, l.created_at
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.agent_id = ? AND l.tenant_id = ?`,
		agentID.String(), tid.String(),
	)
	if err != nil {
		return nil, err
	}
	return scanSQLiteLinks(rows)
}

func (s *SQLiteAgentWorkstationLinkStore) ListForWorkstation(ctx context.Context, workstationID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	tid := store.TenantIDFromContext(ctx)
	if tid == uuid.Nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.agent_id, l.workstation_id, l.tenant_id, l.is_default, l.created_at
		 FROM agent_workstation_links l
		 JOIN agents a ON a.id = l.agent_id AND a.tenant_id = l.tenant_id
		 JOIN workstations w ON w.id = l.workstation_id AND w.tenant_id = l.tenant_id
		 WHERE l.workstation_id = ? AND l.tenant_id = ?`,
		workstationID.String(), tid.String(),
	)
	if err != nil {
		return nil, err
	}
	return scanSQLiteLinks(rows)
}

func scanSQLiteLinks(rows *sql.Rows) ([]store.AgentWorkstationLink, error) {
	defer rows.Close()
	var result []store.AgentWorkstationLink
	for rows.Next() {
		var l store.AgentWorkstationLink
		var agentStr, wsStr, tenantStr string
		var isDefaultInt int
		var createdAt sqliteTime
		if err := rows.Scan(&agentStr, &wsStr, &tenantStr, &isDefaultInt, &createdAt); err != nil {
			continue
		}
		l.AgentID, _ = uuid.Parse(agentStr)
		l.WorkstationID, _ = uuid.Parse(wsStr)
		l.TenantID, _ = uuid.Parse(tenantStr)
		l.IsDefault = isDefaultInt != 0
		l.CreatedAt = createdAt.Time
		result = append(result, l)
	}
	return result, rows.Err()
}
