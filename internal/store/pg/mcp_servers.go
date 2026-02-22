package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/crypto"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGMCPServerStore implements store.MCPServerStore backed by Postgres.
type PGMCPServerStore struct {
	db     *sql.DB
	encKey string // AES-256 encryption key for API keys
}

func NewPGMCPServerStore(db *sql.DB, encryptionKey string) *PGMCPServerStore {
	return &PGMCPServerStore{db: db, encKey: encryptionKey}
}

// --- Server CRUD ---

func (s *PGMCPServerStore) CreateServer(ctx context.Context, srv *store.MCPServerData) error {
	if err := store.ValidateUserID(srv.CreatedBy); err != nil {
		return err
	}
	if srv.ID == uuid.Nil {
		srv.ID = store.GenNewID()
	}

	apiKey := srv.APIKey
	if s.encKey != "" && apiKey != "" {
		encrypted, err := crypto.Encrypt(apiKey, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		apiKey = encrypted
	}

	now := time.Now()
	srv.CreatedAt = now
	srv.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_servers (id, name, display_name, transport, command, args, url, headers, env,
		 api_key, tool_prefix, timeout_sec, settings, enabled, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		srv.ID, srv.Name, nilStr(srv.DisplayName), srv.Transport, nilStr(srv.Command),
		jsonOrEmpty(srv.Args), nilStr(srv.URL), jsonOrEmpty(srv.Headers), jsonOrEmpty(srv.Env),
		nilStr(apiKey), nilStr(srv.ToolPrefix), srv.TimeoutSec,
		jsonOrEmpty(srv.Settings), srv.Enabled, srv.CreatedBy, now, now,
	)
	return err
}

func (s *PGMCPServerStore) GetServer(ctx context.Context, id uuid.UUID) (*store.MCPServerData, error) {
	return s.scanServer(s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, transport, command, args, url, headers, env,
		 api_key, tool_prefix, timeout_sec, settings, enabled, created_by, created_at, updated_at
		 FROM mcp_servers WHERE id = $1`, id))
}

func (s *PGMCPServerStore) GetServerByName(ctx context.Context, name string) (*store.MCPServerData, error) {
	return s.scanServer(s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, transport, command, args, url, headers, env,
		 api_key, tool_prefix, timeout_sec, settings, enabled, created_by, created_at, updated_at
		 FROM mcp_servers WHERE name = $1`, name))
}

func (s *PGMCPServerStore) scanServer(row *sql.Row) (*store.MCPServerData, error) {
	var srv store.MCPServerData
	var displayName, command, url, apiKey, toolPrefix *string
	err := row.Scan(
		&srv.ID, &srv.Name, &displayName, &srv.Transport, &command,
		&srv.Args, &url, &srv.Headers, &srv.Env,
		&apiKey, &toolPrefix, &srv.TimeoutSec,
		&srv.Settings, &srv.Enabled, &srv.CreatedBy, &srv.CreatedAt, &srv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	srv.DisplayName = derefStr(displayName)
	srv.Command = derefStr(command)
	srv.URL = derefStr(url)
	srv.ToolPrefix = derefStr(toolPrefix)
	if apiKey != nil && *apiKey != "" && s.encKey != "" {
		decrypted, err := crypto.Decrypt(*apiKey, s.encKey)
		if err != nil {
			slog.Warn("mcp: failed to decrypt api key", "server", srv.Name, "error", err)
		} else {
			srv.APIKey = decrypted
		}
	} else {
		srv.APIKey = derefStr(apiKey)
	}
	return &srv, nil
}

func (s *PGMCPServerStore) ListServers(ctx context.Context) ([]store.MCPServerData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name, transport, command, args, url, headers, env,
		 api_key, tool_prefix, timeout_sec, settings, enabled, created_by, created_at, updated_at
		 FROM mcp_servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.MCPServerData
	for rows.Next() {
		var srv store.MCPServerData
		var displayName, command, url, apiKey, toolPrefix *string
		if err := rows.Scan(
			&srv.ID, &srv.Name, &displayName, &srv.Transport, &command,
			&srv.Args, &url, &srv.Headers, &srv.Env,
			&apiKey, &toolPrefix, &srv.TimeoutSec,
			&srv.Settings, &srv.Enabled, &srv.CreatedBy, &srv.CreatedAt, &srv.UpdatedAt,
		); err != nil {
			continue
		}
		srv.DisplayName = derefStr(displayName)
		srv.Command = derefStr(command)
		srv.URL = derefStr(url)
		srv.ToolPrefix = derefStr(toolPrefix)
		if apiKey != nil && *apiKey != "" && s.encKey != "" {
			if decrypted, err := crypto.Decrypt(*apiKey, s.encKey); err == nil {
				srv.APIKey = decrypted
			}
		} else {
			srv.APIKey = derefStr(apiKey)
		}
		result = append(result, srv)
	}
	return result, nil
}

func (s *PGMCPServerStore) UpdateServer(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	// Encrypt api_key if present
	if key, ok := updates["api_key"]; ok {
		if keyStr, isStr := key.(string); isStr && keyStr != "" && s.encKey != "" {
			encrypted, err := crypto.Encrypt(keyStr, s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt api key: %w", err)
			}
			updates["api_key"] = encrypted
		}
	}
	updates["updated_at"] = time.Now()
	return execMapUpdate(ctx, s.db, "mcp_servers", id, updates)
}

func (s *PGMCPServerStore) DeleteServer(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_servers WHERE id = $1", id)
	return err
}

// --- Agent Grants ---

func (s *PGMCPServerStore) GrantToAgent(ctx context.Context, g *store.MCPAgentGrant) error {
	if err := store.ValidateUserID(g.GrantedBy); err != nil {
		return err
	}
	if g.ID == uuid.Nil {
		g.ID = store.GenNewID()
	}
	g.CreatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_agent_grants (id, server_id, agent_id, enabled, tool_allow, tool_deny, config_overrides, granted_by, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (server_id, agent_id) DO UPDATE SET
		   enabled = EXCLUDED.enabled, tool_allow = EXCLUDED.tool_allow,
		   tool_deny = EXCLUDED.tool_deny, config_overrides = EXCLUDED.config_overrides,
		   granted_by = EXCLUDED.granted_by`,
		g.ID, g.ServerID, g.AgentID, g.Enabled,
		jsonOrNull(g.ToolAllow), jsonOrNull(g.ToolDeny), jsonOrNull(g.ConfigOverrides),
		g.GrantedBy, g.CreatedAt,
	)
	return err
}

func (s *PGMCPServerStore) RevokeFromAgent(ctx context.Context, serverID, agentID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM mcp_agent_grants WHERE server_id = $1 AND agent_id = $2", serverID, agentID)
	return err
}

func (s *PGMCPServerStore) ListAgentGrants(ctx context.Context, agentID uuid.UUID) ([]store.MCPAgentGrant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, server_id, agent_id, enabled, tool_allow, tool_deny, config_overrides, granted_by, created_at
		 FROM mcp_agent_grants WHERE agent_id = $1`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.MCPAgentGrant
	for rows.Next() {
		var g store.MCPAgentGrant
		if err := rows.Scan(&g.ID, &g.ServerID, &g.AgentID, &g.Enabled,
			&g.ToolAllow, &g.ToolDeny, &g.ConfigOverrides, &g.GrantedBy, &g.CreatedAt); err != nil {
			continue
		}
		result = append(result, g)
	}
	return result, nil
}

// --- User Grants ---

func (s *PGMCPServerStore) GrantToUser(ctx context.Context, g *store.MCPUserGrant) error {
	if err := store.ValidateUserID(g.UserID); err != nil {
		return err
	}
	if err := store.ValidateUserID(g.GrantedBy); err != nil {
		return err
	}
	if g.ID == uuid.Nil {
		g.ID = store.GenNewID()
	}
	g.CreatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_user_grants (id, server_id, user_id, enabled, tool_allow, tool_deny, granted_by, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (server_id, user_id) DO UPDATE SET
		   enabled = EXCLUDED.enabled, tool_allow = EXCLUDED.tool_allow,
		   tool_deny = EXCLUDED.tool_deny, granted_by = EXCLUDED.granted_by`,
		g.ID, g.ServerID, g.UserID, g.Enabled,
		jsonOrNull(g.ToolAllow), jsonOrNull(g.ToolDeny),
		g.GrantedBy, g.CreatedAt,
	)
	return err
}

func (s *PGMCPServerStore) RevokeFromUser(ctx context.Context, serverID uuid.UUID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM mcp_user_grants WHERE server_id = $1 AND user_id = $2", serverID, userID)
	return err
}

// --- Resolution ---

func (s *PGMCPServerStore) ListAccessible(ctx context.Context, agentID uuid.UUID, userID string) ([]store.MCPAccessInfo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ms.id, ms.name, ms.display_name, ms.transport, ms.command, ms.args, ms.url, ms.headers, ms.env,
		 ms.api_key, ms.tool_prefix, ms.timeout_sec, ms.settings, ms.enabled, ms.created_by, ms.created_at, ms.updated_at,
		 mag.tool_allow, mag.tool_deny
		 FROM mcp_servers ms
		 INNER JOIN mcp_agent_grants mag ON ms.id = mag.server_id AND mag.agent_id = $1 AND mag.enabled = true
		 LEFT JOIN mcp_user_grants mug ON ms.id = mug.server_id AND mug.user_id = $2
		 WHERE ms.enabled = true
		   AND (mug.id IS NULL OR mug.enabled = true)`,
		agentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.MCPAccessInfo
	for rows.Next() {
		var srv store.MCPServerData
		var displayName, command, url, apiKey, toolPrefix *string
		var toolAllowJSON, toolDenyJSON []byte

		if err := rows.Scan(
			&srv.ID, &srv.Name, &displayName, &srv.Transport, &command,
			&srv.Args, &url, &srv.Headers, &srv.Env,
			&apiKey, &toolPrefix, &srv.TimeoutSec,
			&srv.Settings, &srv.Enabled, &srv.CreatedBy, &srv.CreatedAt, &srv.UpdatedAt,
			&toolAllowJSON, &toolDenyJSON,
		); err != nil {
			continue
		}
		srv.DisplayName = derefStr(displayName)
		srv.Command = derefStr(command)
		srv.URL = derefStr(url)
		srv.ToolPrefix = derefStr(toolPrefix)
		if apiKey != nil && *apiKey != "" && s.encKey != "" {
			if decrypted, err := crypto.Decrypt(*apiKey, s.encKey); err == nil {
				srv.APIKey = decrypted
			}
		} else {
			srv.APIKey = derefStr(apiKey)
		}

		info := store.MCPAccessInfo{Server: srv}
		if toolAllowJSON != nil {
			json.Unmarshal(toolAllowJSON, &info.ToolAllow)
		}
		if toolDenyJSON != nil {
			json.Unmarshal(toolDenyJSON, &info.ToolDeny)
		}
		result = append(result, info)
	}
	return result, nil
}

// --- Access Requests ---

func (s *PGMCPServerStore) CreateRequest(ctx context.Context, req *store.MCPAccessRequest) error {
	if err := store.ValidateUserID(req.RequestedBy); err != nil {
		return err
	}
	if req.ID == uuid.Nil {
		req.ID = store.GenNewID()
	}
	req.Status = "pending"
	req.CreatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mcp_access_requests (id, server_id, agent_id, user_id, scope, status, reason, tool_allow, requested_by, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		req.ID, req.ServerID, nilUUID(req.AgentID), nilStr(req.UserID),
		req.Scope, req.Status, nilStr(req.Reason),
		jsonOrNull(req.ToolAllow), req.RequestedBy, req.CreatedAt,
	)
	return err
}

func (s *PGMCPServerStore) ListPendingRequests(ctx context.Context) ([]store.MCPAccessRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, server_id, agent_id, user_id, scope, status, reason, tool_allow, requested_by,
		 reviewed_by, reviewed_at, review_note, created_at
		 FROM mcp_access_requests WHERE status = 'pending' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.MCPAccessRequest
	for rows.Next() {
		var r store.MCPAccessRequest
		var agentID *uuid.UUID
		var userID, reviewedBy, reviewNote *string
		if err := rows.Scan(&r.ID, &r.ServerID, &agentID, &userID, &r.Scope, &r.Status,
			&r.Reason, &r.ToolAllow, &r.RequestedBy,
			&reviewedBy, &r.ReviewedAt, &reviewNote, &r.CreatedAt); err != nil {
			continue
		}
		r.AgentID = agentID
		r.UserID = derefStr(userID)
		r.ReviewedBy = derefStr(reviewedBy)
		r.ReviewNote = derefStr(reviewNote)
		result = append(result, r)
	}
	return result, nil
}

func (s *PGMCPServerStore) ReviewRequest(ctx context.Context, requestID uuid.UUID, approved bool, reviewedBy, note string) error {
	if err := store.ValidateUserID(reviewedBy); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Load the request
	var req store.MCPAccessRequest
	var agentID *uuid.UUID
	var userID *string
	err = tx.QueryRowContext(ctx,
		`SELECT id, server_id, agent_id, user_id, scope, status, tool_allow
		 FROM mcp_access_requests WHERE id = $1 AND status = 'pending'`, requestID,
	).Scan(&req.ID, &req.ServerID, &agentID, &userID, &req.Scope, &req.Status, &req.ToolAllow)
	if err != nil {
		return fmt.Errorf("request not found or not pending: %w", err)
	}

	status := "rejected"
	if approved {
		status = "approved"
	}
	now := time.Now()

	// Update request status
	_, err = tx.ExecContext(ctx,
		`UPDATE mcp_access_requests SET status = $1, reviewed_by = $2, reviewed_at = $3, review_note = $4 WHERE id = $5`,
		status, reviewedBy, now, nilStr(note), requestID,
	)
	if err != nil {
		return err
	}

	// If approved, insert the grant
	if approved {
		switch req.Scope {
		case "agent":
			if agentID == nil {
				return fmt.Errorf("agent_id required for agent scope")
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO mcp_agent_grants (id, server_id, agent_id, enabled, tool_allow, granted_by, created_at)
				 VALUES ($1,$2,$3,true,$4,$5,$6)
				 ON CONFLICT (server_id, agent_id) DO UPDATE SET enabled = true, tool_allow = EXCLUDED.tool_allow, granted_by = EXCLUDED.granted_by`,
				store.GenNewID(), req.ServerID, *agentID, jsonOrNull(req.ToolAllow), reviewedBy, now,
			)
		case "user":
			if userID == nil || *userID == "" {
				return fmt.Errorf("user_id required for user scope")
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO mcp_user_grants (id, server_id, user_id, enabled, tool_allow, granted_by, created_at)
				 VALUES ($1,$2,$3,true,$4,$5,$6)
				 ON CONFLICT (server_id, user_id) DO UPDATE SET enabled = true, tool_allow = EXCLUDED.tool_allow, granted_by = EXCLUDED.granted_by`,
				store.GenNewID(), req.ServerID, *userID, jsonOrNull(req.ToolAllow), reviewedBy, now,
			)
		default:
			return fmt.Errorf("unknown scope: %s", req.Scope)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
