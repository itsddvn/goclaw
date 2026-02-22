package store

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	// UserIDKey is the context key for the external user ID (TEXT, free-form).
	UserIDKey contextKey = "goclaw_user_id"
	// AgentIDKey is the context key for the agent UUID (managed mode).
	AgentIDKey contextKey = "goclaw_agent_id"
	// AgentTypeKey is the context key for the agent type ("open" or "predefined").
	AgentTypeKey contextKey = "goclaw_agent_type"
)

// WithUserID returns a new context with the given user ID.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDKey, id)
}

// UserIDFromContext extracts the user ID from context. Returns "" if not set.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// WithAgentID returns a new context with the given agent UUID.
func WithAgentID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, AgentIDKey, id)
}

// AgentIDFromContext extracts the agent UUID from context. Returns uuid.Nil if not set.
func AgentIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(AgentIDKey).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// WithAgentType returns a new context with the given agent type.
func WithAgentType(ctx context.Context, t string) context.Context {
	return context.WithValue(ctx, AgentTypeKey, t)
}

// AgentTypeFromContext extracts the agent type from context. Returns "" if not set.
func AgentTypeFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(AgentTypeKey).(string); ok {
		return v
	}
	return ""
}
