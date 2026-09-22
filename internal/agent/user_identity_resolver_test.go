package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// mockResolver is a test double that returns pre-configured resolutions.
type mockResolver struct {
	mergedMap  map[string]string    // "channelType:senderID" → resolved tenant user ID
	contactMap map[string]uuid.UUID // "channelType:senderID" → exact Contact UUID
}

func (m *mockResolver) ResolveTenantUserID(_ context.Context, channelType, senderID string) (string, error) {
	key := channelType + ":" + senderID
	if resolved, ok := m.mergedMap[key]; ok {
		return resolved, nil
	}
	return "", nil
}

func (m *mockResolver) ResolveContactID(_ context.Context, channelType, senderID string) (uuid.UUID, error) {
	key := channelType + ":" + senderID
	if resolved, ok := m.contactMap[key]; ok {
		return resolved, nil
	}
	return uuid.Nil, nil
}

func TestResolveCredentialUserID(t *testing.T) {
	tests := []struct {
		name       string
		req        RunRequest
		mergedMap  map[string]string // nil signals nil resolver
		wantUserID string
	}{
		// --- DM scenarios ---
		{
			name:       "DM: sender merged to tenant user",
			req:        RunRequest{UserID: "12345", ChannelType: "telegram", PeerKind: "direct"},
			mergedMap:  map[string]string{"telegram:12345": "john@co.com"},
			wantUserID: "john@co.com",
		},
		{
			name:       "DM: sender not merged — returns raw UserID",
			req:        RunRequest{UserID: "12345", ChannelType: "telegram", PeerKind: "direct"},
			mergedMap:  map[string]string{},
			wantUserID: "12345",
		},
		{
			name:       "DM: already resolved by consumer_normal — no double resolve",
			req:        RunRequest{UserID: "john@co.com", ChannelType: "telegram", PeerKind: "direct"},
			mergedMap:  map[string]string{},
			wantUserID: "john@co.com",
		},

		// --- Group scenarios ---
		{
			name:       "Group: individual sender merged",
			req:        RunRequest{UserID: "group:tg:-100456", SenderID: "12345", ChannelType: "telegram", PeerKind: "group"},
			mergedMap:  map[string]string{"telegram:12345": "john@co.com"},
			wantUserID: "john@co.com",
		},
		{
			name:       "Group: group contact merged (sender not merged)",
			req:        RunRequest{UserID: "group:tg:-100456", SenderID: "99999", ChannelType: "telegram", PeerKind: "group"},
			mergedMap:  map[string]string{"telegram:-100456": "team-acme@co.com"},
			wantUserID: "team-acme@co.com",
		},
		{
			name: "Group: both sender and group merged — group takes priority",
			req:  RunRequest{UserID: "group:tg:-100456", SenderID: "12345", ChannelType: "telegram", PeerKind: "group"},
			mergedMap: map[string]string{
				"telegram:12345":   "john@co.com",
				"telegram:-100456": "team-acme@co.com",
			},
			wantUserID: "team-acme@co.com", // group > individual
		},
		{
			name:       "Group: nobody merged — returns group composite",
			req:        RunRequest{UserID: "group:tg:-100456", SenderID: "99999", ChannelType: "telegram", PeerKind: "group"},
			mergedMap:  map[string]string{},
			wantUserID: "group:tg:-100456",
		},

		// --- WS/HTTP/cron scenarios ---
		{
			name:       "WS chat: no channelType — skip resolve, return UserID",
			req:        RunRequest{UserID: "john@co.com", PeerKind: ""},
			mergedMap:  map[string]string{},
			wantUserID: "john@co.com",
		},
		{
			name:       "Cron: no channelType — skip resolve",
			req:        RunRequest{UserID: "12345"},
			mergedMap:  map[string]string{},
			wantUserID: "12345",
		},

		// --- Edge cases ---
		{
			name:       "SenderID with pipe suffix stripped",
			req:        RunRequest{UserID: "group:tg:-100456", SenderID: "12345|username", ChannelType: "telegram", PeerKind: "group"},
			mergedMap:  map[string]string{"telegram:12345": "john@co.com"},
			wantUserID: "john@co.com",
		},
		{
			name:       "Nil resolver — returns raw UserID",
			req:        RunRequest{UserID: "group:tg:-100456", SenderID: "12345", ChannelType: "telegram", PeerKind: "group"},
			mergedMap:  nil, // signals nil resolver
			wantUserID: "group:tg:-100456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loop := &Loop{}
			if tt.mergedMap != nil {
				loop.userResolver = &mockResolver{mergedMap: tt.mergedMap}
			}

			got := loop.resolveCredentialUserID(context.Background(), tt.req)
			if got != tt.wantUserID {
				t.Errorf("resolveCredentialUserID() = %q, want %q", got, tt.wantUserID)
			}
		})
	}
}

func TestResolveWorkstationContactID(t *testing.T) {
	contactA := uuid.New()
	loop := &Loop{userResolver: &mockResolver{
		mergedMap: map[string]string{
			"telegram:-100456": "shared-tenant-user",
			"telegram:12345":   "shared-tenant-user",
		},
		contactMap: map[string]uuid.UUID{
			"telegram:12345": contactA,
		},
	}}

	t.Run("group uses exact individual Contact, not shared merged identity", func(t *testing.T) {
		req := RunRequest{
			UserID:      "group:telegram:-100456",
			SenderID:    "12345|display-name",
			ChannelType: "telegram",
			PeerKind:    "group",
		}
		if got := loop.resolveWorkstationContactID(context.Background(), req); got != contactA {
			t.Fatalf("resolveWorkstationContactID() = %s, want %s", got, contactA)
		}
	})

	t.Run("group without attributable sender denies by returning empty", func(t *testing.T) {
		req := RunRequest{
			UserID:      "group:telegram:-100456",
			ChannelType: "telegram",
			PeerKind:    "group",
		}
		if got := loop.resolveWorkstationContactID(context.Background(), req); got != uuid.Nil {
			t.Fatalf("resolveWorkstationContactID() = %s, want nil UUID", got)
		}
	})

	t.Run("direct message resolves exact Contact instead of tenant user", func(t *testing.T) {
		req := RunRequest{UserID: "12345", ChannelType: "telegram", PeerKind: "direct"}
		if got := loop.resolveWorkstationContactID(context.Background(), req); got != contactA {
			t.Fatalf("resolveWorkstationContactID() = %s, want %s", got, contactA)
		}
	})
}

func TestExtractGroupChatID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"group:telegram:-100456", "-100456"},
		{"group:discord:123", "123"},
		{"guild:123:user:456", ""}, // not group: prefix
		{"12345", ""},              // plain user ID
		{"group:", ""},             // incomplete
		{"group:channel", ""},      // missing third part
		{"group:tg:-100456", "-100456"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%s", tt.input), func(t *testing.T) {
			got := extractGroupChatID(tt.input)
			if got != tt.want {
				t.Errorf("extractGroupChatID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCredentialUserIDFromContext(t *testing.T) {
	t.Run("both set — returns CredentialUserID", func(t *testing.T) {
		ctx := store.WithUserID(context.Background(), "raw-user")
		ctx = store.WithCredentialUserID(ctx, "resolved-user")
		got := store.CredentialUserIDFromContext(ctx)
		if got != "resolved-user" {
			t.Errorf("got %q, want %q", got, "resolved-user")
		}
	})

	t.Run("only UserID set — falls back to UserID", func(t *testing.T) {
		ctx := store.WithUserID(context.Background(), "raw-user")
		got := store.CredentialUserIDFromContext(ctx)
		if got != "raw-user" {
			t.Errorf("got %q, want %q", got, "raw-user")
		}
	})

	t.Run("neither set — returns empty", func(t *testing.T) {
		got := store.CredentialUserIDFromContext(context.Background())
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestWorkstationContactIDFromContext(t *testing.T) {
	contactID := uuid.New()
	t.Run("explicit Contact is independent from shared credential identity", func(t *testing.T) {
		ctx := store.WithCredentialUserID(context.Background(), "shared-group-user")
		ctx = store.WithWorkstationContactID(ctx, contactID)
		if got := store.WorkstationContactIDFromContext(ctx); got != contactID {
			t.Fatalf("got %s, want %s", got, contactID)
		}
	})

	t.Run("direct caller does not fall back to credential identity", func(t *testing.T) {
		ctx := store.WithCredentialUserID(context.Background(), "direct-user")
		if got := store.WorkstationContactIDFromContext(ctx); got != uuid.Nil {
			t.Fatalf("got %s, want nil UUID", got)
		}
	})

	t.Run("resolved empty principal remains denied after run context restoration", func(t *testing.T) {
		ctx := store.WithCredentialUserID(context.Background(), "shared-group-user")
		ctx = store.WithRunContext(ctx, &store.RunContext{
			CredentialUserID:           "shared-group-user",
			WorkstationContactResolved: true,
		})
		if got := store.WorkstationContactIDFromContext(ctx); got != uuid.Nil {
			t.Fatalf("got %s, want nil UUID denied principal", got)
		}
	})
}
