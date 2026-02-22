package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGAgentStore implements store.AgentStore backed by Postgres.
type PGAgentStore struct {
	db *sql.DB
}

func NewPGAgentStore(db *sql.DB) *PGAgentStore {
	return &PGAgentStore{db: db}
}

// agentSelectCols is the column list for all agent SELECT queries.
const agentSelectCols = `id, agent_key, display_name, owner_id, provider, model,
		 context_window, max_tool_iterations, workspace, restrict_to_workspace,
		 tools_config, sandbox_config, subagents_config, memory_config,
		 compaction_config, context_pruning, other_config,
		 agent_type, is_default, status, created_at, updated_at`

func (s *PGAgentStore) Create(ctx context.Context, agent *store.AgentData) error {
	if agent.ID == uuid.Nil {
		agent.ID = store.GenNewID()
	}
	now := time.Now()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agents (id, agent_key, display_name, owner_id, provider, model,
		 context_window, max_tool_iterations, workspace, restrict_to_workspace,
		 tools_config, sandbox_config, subagents_config, memory_config,
		 compaction_config, context_pruning, other_config,
		 agent_type, is_default, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		agent.ID, agent.AgentKey, agent.DisplayName, agent.OwnerID, agent.Provider, agent.Model,
		agent.ContextWindow, agent.MaxToolIterations, agent.Workspace, agent.RestrictToWorkspace,
		jsonOrEmpty(agent.ToolsConfig), jsonOrNull(agent.SandboxConfig), jsonOrNull(agent.SubagentsConfig), jsonOrNull(agent.MemoryConfig),
		jsonOrNull(agent.CompactionConfig), jsonOrNull(agent.ContextPruning), jsonOrEmpty(agent.OtherConfig),
		agent.AgentType, agent.IsDefault, agent.Status, now, now,
	)
	return err
}

func (s *PGAgentStore) GetByKey(ctx context.Context, agentKey string) (*store.AgentData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+agentSelectCols+`
		 FROM agents WHERE agent_key = $1 AND deleted_at IS NULL`, agentKey)
	d, err := scanAgentRow(row)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", agentKey)
	}
	return d, nil
}

func (s *PGAgentStore) GetByID(ctx context.Context, id uuid.UUID) (*store.AgentData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+agentSelectCols+`
		 FROM agents WHERE id = $1 AND deleted_at IS NULL`, id)
	d, err := scanAgentRow(row)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	return d, nil
}

func (s *PGAgentStore) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	// Add soft-delete guard to WHERE clause
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()
	return execMapUpdateWhere(ctx, s.db, "agents", updates, "id = $IDX AND deleted_at IS NULL", id)
}

func (s *PGAgentStore) Delete(ctx context.Context, id uuid.UUID) error {
	// Soft delete
	_, err := s.db.ExecContext(ctx, "UPDATE agents SET deleted_at = NOW() WHERE id = $1", id)
	return err
}

func (s *PGAgentStore) List(ctx context.Context, ownerID string) ([]store.AgentData, error) {
	var rows *sql.Rows
	var err error
	if ownerID != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+agentSelectCols+`
			 FROM agents WHERE deleted_at IS NULL AND owner_id = $1 ORDER BY created_at DESC`, ownerID)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+agentSelectCols+`
			 FROM agents WHERE deleted_at IS NULL ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgentRows(rows)
}

// --- Access Control ---

func (s *PGAgentStore) ShareAgent(ctx context.Context, agentID uuid.UUID, userID, role, grantedBy string) error {
	if err := store.ValidateUserID(userID); err != nil {
		return err
	}
	if err := store.ValidateUserID(grantedBy); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_shares (id, agent_id, user_id, role, granted_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (agent_id, user_id) DO UPDATE SET role = EXCLUDED.role, granted_by = EXCLUDED.granted_by`,
		store.GenNewID(), agentID, userID, role, grantedBy, time.Now(),
	)
	return err
}

func (s *PGAgentStore) RevokeShare(ctx context.Context, agentID uuid.UUID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM agent_shares WHERE agent_id = $1 AND user_id = $2", agentID, userID)
	return err
}

func (s *PGAgentStore) ListShares(ctx context.Context, agentID uuid.UUID) ([]store.AgentShareData, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, agent_id, user_id, role, granted_by, created_at FROM agent_shares WHERE agent_id = $1", agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.AgentShareData
	for rows.Next() {
		var d store.AgentShareData
		if err := rows.Scan(&d.ID, &d.AgentID, &d.UserID, &d.Role, &d.GrantedBy, &d.CreatedAt); err != nil {
			continue
		}
		result = append(result, d)
	}
	return result, nil
}

func (s *PGAgentStore) CanAccess(ctx context.Context, agentID uuid.UUID, userID string) (bool, string, error) {
	// Check ownership + default flag
	var ownerID string
	var isDefault bool
	err := s.db.QueryRowContext(ctx,
		"SELECT owner_id, is_default FROM agents WHERE id = $1 AND deleted_at IS NULL", agentID,
	).Scan(&ownerID, &isDefault)
	if err != nil {
		return false, "", fmt.Errorf("agent not found")
	}
	if isDefault {
		if ownerID == userID {
			return true, "owner", nil
		}
		return true, "user", nil
	}
	if ownerID == userID {
		return true, "owner", nil
	}
	// Check shares
	var role string
	err = s.db.QueryRowContext(ctx,
		"SELECT role FROM agent_shares WHERE agent_id = $1 AND user_id = $2", agentID, userID,
	).Scan(&role)
	if err != nil {
		return false, "", nil
	}
	return true, role, nil
}

func (s *PGAgentStore) ListAccessible(ctx context.Context, userID string) ([]store.AgentData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+agentSelectCols+`
		 FROM agents
		 WHERE deleted_at IS NULL AND (owner_id = $1 OR is_default = true OR id IN (SELECT agent_id FROM agent_shares WHERE user_id = $1))
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgentRows(rows)
}

// --- Scan helpers ---

type agentRowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgentRow(row agentRowScanner) (*store.AgentData, error) {
	var d store.AgentData
	// pgx: scan nullable JSONB into *[]byte (NOT *json.RawMessage — pgx can't scan NULL into defined types)
	var toolsCfg, sandboxCfg, subagentsCfg, memoryCfg, compactionCfg, pruningCfg, otherCfg *[]byte
	err := row.Scan(&d.ID, &d.AgentKey, &d.DisplayName, &d.OwnerID, &d.Provider, &d.Model,
		&d.ContextWindow, &d.MaxToolIterations, &d.Workspace, &d.RestrictToWorkspace,
		&toolsCfg, &sandboxCfg, &subagentsCfg, &memoryCfg, &compactionCfg, &pruningCfg, &otherCfg,
		&d.AgentType, &d.IsDefault, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// Convert *[]byte → json.RawMessage (nil-safe)
	if toolsCfg != nil {
		d.ToolsConfig = *toolsCfg
	}
	if sandboxCfg != nil {
		d.SandboxConfig = *sandboxCfg
	}
	if subagentsCfg != nil {
		d.SubagentsConfig = *subagentsCfg
	}
	if memoryCfg != nil {
		d.MemoryConfig = *memoryCfg
	}
	if compactionCfg != nil {
		d.CompactionConfig = *compactionCfg
	}
	if pruningCfg != nil {
		d.ContextPruning = *pruningCfg
	}
	if otherCfg != nil {
		d.OtherConfig = *otherCfg
	}
	return &d, nil
}

func scanAgentRows(rows *sql.Rows) ([]store.AgentData, error) {
	var result []store.AgentData
	for rows.Next() {
		d, err := scanAgentRow(rows)
		if err != nil {
			continue
		}
		result = append(result, *d)
	}
	return result, nil
}

// execMapUpdateWhere is like execMapUpdate but with a custom WHERE clause.
// The whereClause should use $IDX as placeholder for the ID (will be replaced with the next arg index).
func execMapUpdateWhere(ctx context.Context, db *sql.DB, table string, updates map[string]any, whereClause string, id uuid.UUID) error {
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []interface{}
	i := 1
	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	// Replace $IDX with the actual parameter index for the id
	where := fmt.Sprintf(whereClause[0:0]+"%s", whereClause)
	finalWhere := ""
	for _, ch := range where {
		finalWhere += string(ch)
	}
	// Simple replace: $IDX → $N
	idxStr := fmt.Sprintf("$%d", i)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		table,
		joinStrings(setClauses, ", "),
		replaceIDX(whereClause, idxStr))
	_, err := db.ExecContext(ctx, q, args...)
	return err
}

func joinStrings(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}

func replaceIDX(s, replacement string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+4 <= len(s) && s[i:i+4] == "$IDX" {
			result += replacement
			i += 3
		} else {
			result += string(s[i])
		}
	}
	return result
}
