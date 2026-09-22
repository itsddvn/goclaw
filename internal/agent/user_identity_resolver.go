package agent

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// UserIdentityResolver resolves raw user IDs to merged tenant user identities.
// Used by the agent loop to set CredentialUserID in context before tool execution.
type UserIdentityResolver interface {
	ResolveTenantUserID(ctx context.Context, channelType, senderID string) (string, error)
	ResolveContactID(ctx context.Context, channelType, senderID string) (uuid.UUID, error)
}

// contactStoreResolver wraps a ContactStore for nil-safe resolution.
type contactStoreResolver struct {
	store interface {
		ResolveTenantUserID(ctx context.Context, channelType, senderID string) (string, error)
		ResolveContactID(ctx context.Context, channelType, senderID string) (uuid.UUID, error)
	}
}

func (r *contactStoreResolver) ResolveContactID(ctx context.Context, channelType, senderID string) (uuid.UUID, error) {
	if r.store == nil {
		return uuid.Nil, nil
	}
	return r.store.ResolveContactID(ctx, channelType, senderID)
}

func (r *contactStoreResolver) ResolveTenantUserID(ctx context.Context, channelType, senderID string) (string, error) {
	if r.store == nil {
		return "", nil
	}
	return r.store.ResolveTenantUserID(ctx, channelType, senderID)
}

// newContactResolver creates a UserIdentityResolver from a ContactStore.
// Returns nil if the store is nil (resolver is optional).
func newContactResolver(cs interface {
	ResolveTenantUserID(ctx context.Context, channelType, senderID string) (string, error)
	ResolveContactID(ctx context.Context, channelType, senderID string) (uuid.UUID, error)
}) UserIdentityResolver {
	if cs == nil {
		return nil
	}
	return &contactStoreResolver{store: cs}
}

// resolveCredentialUserID determines the best tenant user identity for
// credential lookups. For DMs, tries resolving the user ID directly.
// For groups, tries the individual sender first, then the group contact.
func (l *Loop) resolveCredentialUserID(ctx context.Context, req RunRequest) string {
	if l.userResolver == nil {
		return req.UserID
	}

	channelType := req.ChannelType

	// Non-group: try resolving UserID directly (covers unresolved DMs, HTTP API, cron)
	if req.PeerKind != "group" {
		if channelType != "" && req.UserID != "" {
			resolved, err := l.userResolver.ResolveTenantUserID(ctx, channelType, req.UserID)
			if err != nil {
				slog.Debug("credential_resolve.dm_failed", "user", req.UserID, "channel", channelType, "error", err)
			} else if resolved != "" {
				return resolved
			}
		}
		return req.UserID
	}

	// Group: try group contact first (group has its own credentials),
	// fall back to individual sender if no group credential exists.
	if chatID := extractGroupChatID(req.UserID); chatID != "" && channelType != "" {
		resolved, err := l.userResolver.ResolveTenantUserID(ctx, channelType, chatID)
		if err != nil {
			slog.Debug("credential_resolve.group_contact_failed", "chatID", chatID, "channel", channelType, "error", err)
		} else if resolved != "" {
			return resolved
		}
	}

	if req.SenderID != "" && channelType != "" {
		senderNumeric := req.SenderID
		if idx := strings.IndexByte(senderNumeric, '|'); idx > 0 {
			senderNumeric = senderNumeric[:idx]
		}
		resolved, err := l.userResolver.ResolveTenantUserID(ctx, channelType, senderNumeric)
		if err != nil {
			slog.Debug("credential_resolve.group_sender_failed", "sender", senderNumeric, "channel", channelType, "error", err)
		} else if resolved != "" {
			return resolved
		}
	}

	return req.UserID
}

// resolveWorkstationContactID resolves the exact Contact instructing the agent.
// It deliberately does not fall back to a raw sender or merged tenant user.
func (l *Loop) resolveWorkstationContactID(ctx context.Context, req RunRequest) uuid.UUID {
	senderID := req.UserID
	if req.PeerKind == "group" || strings.HasPrefix(req.UserID, "group:") || strings.HasPrefix(req.UserID, "guild:") {
		senderID = req.SenderID
	}
	if idx := strings.IndexByte(senderID, '|'); idx > 0 {
		senderID = senderID[:idx]
	}
	if senderID == "" || l.userResolver == nil || req.ChannelType == "" {
		return uuid.Nil
	}
	resolved, err := l.userResolver.ResolveContactID(ctx, req.ChannelType, senderID)
	if err != nil {
		slog.Debug("workstation_contact_resolve.sender_failed", "sender", senderID, "channel", req.ChannelType, "error", err)
		return uuid.Nil
	}
	return resolved
}

// extractGroupChatID extracts the chat ID from a group composite user ID.
// Format: "group:{channel}:{chatID}" → returns chatID.
// Returns "" if format doesn't match.
func extractGroupChatID(userID string) string {
	if !strings.HasPrefix(userID, "group:") {
		return ""
	}
	parts := strings.SplitN(userID, ":", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
