package tools

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type workstationResolverStore struct {
	items map[uuid.UUID]*store.Workstation
}

func (s *workstationResolverStore) Create(_ context.Context, _ *store.Workstation) error {
	return nil
}

func (s *workstationResolverStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Workstation, error) {
	ws, ok := s.items[id]
	if !ok || ws.TenantID != store.TenantIDFromContext(ctx) {
		return nil, sql.ErrNoRows
	}
	return ws, nil
}

func (s *workstationResolverStore) GetByKey(ctx context.Context, key string) (*store.Workstation, error) {
	for _, ws := range s.items {
		if ws.WorkstationKey == key && ws.TenantID == store.TenantIDFromContext(ctx) {
			return ws, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *workstationResolverStore) List(_ context.Context) ([]store.Workstation, error) {
	return nil, nil
}

func (s *workstationResolverStore) Update(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}

func (s *workstationResolverStore) SetActive(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}

func (s *workstationResolverStore) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

type workstationResolverLinkStore struct {
	links []store.AgentWorkstationLink
	err   error
}

type workstationResolverContactGrantStore struct {
	allowed     bool
	err         error
	calls       int
	lastContact uuid.UUID
}

func (s *workstationResolverContactGrantStore) Grant(_ context.Context, _ *store.WorkstationContactGrant) error {
	return nil
}

func (s *workstationResolverContactGrantStore) Revoke(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (s *workstationResolverContactGrantStore) HasAccess(_ context.Context, _ uuid.UUID, contactID uuid.UUID) (bool, error) {
	s.calls++
	s.lastContact = contactID
	return s.allowed, s.err
}

func (s *workstationResolverContactGrantStore) ListForWorkstation(_ context.Context, _ uuid.UUID) ([]store.WorkstationContactGrant, error) {
	return nil, nil
}

func (s *workstationResolverLinkStore) Link(_ context.Context, link *store.AgentWorkstationLink) error {
	s.links = append(s.links, *link)
	return nil
}

func (s *workstationResolverLinkStore) Unlink(_ context.Context, agentID, workstationID uuid.UUID) error {
	filtered := s.links[:0]
	for _, link := range s.links {
		if link.AgentID != agentID || link.WorkstationID != workstationID {
			filtered = append(filtered, link)
		}
	}
	s.links = filtered
	return nil
}

func (s *workstationResolverLinkStore) HasAccess(ctx context.Context, agentID, workstationID uuid.UUID) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	tid := store.TenantIDFromContext(ctx)
	for _, link := range s.links {
		if link.AgentID == agentID && link.WorkstationID == workstationID && link.TenantID == tid {
			return true, nil
		}
	}
	return false, nil
}

func (s *workstationResolverLinkStore) SetDefault(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (s *workstationResolverLinkStore) ListForAgent(ctx context.Context, agentID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	if s.err != nil {
		return nil, s.err
	}
	tid := store.TenantIDFromContext(ctx)
	result := make([]store.AgentWorkstationLink, 0)
	for _, link := range s.links {
		if link.AgentID == agentID && link.TenantID == tid {
			result = append(result, link)
		}
	}
	return result, nil
}

func (s *workstationResolverLinkStore) ListForWorkstation(ctx context.Context, workstationID uuid.UUID) ([]store.AgentWorkstationLink, error) {
	tid := store.TenantIDFromContext(ctx)
	result := make([]store.AgentWorkstationLink, 0)
	for _, link := range s.links {
		if link.WorkstationID == workstationID && link.TenantID == tid {
			result = append(result, link)
		}
	}
	return result, nil
}

type blockingWorkstationStream struct {
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	stderrR *io.PipeReader
	stderrW *io.PipeWriter
	killN   atomic.Int64
	once    sync.Once
	done    chan struct{}
}

func newBlockingWorkstationStream() *blockingWorkstationStream {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	return &blockingWorkstationStream{
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		stderrR: stderrR,
		stderrW: stderrW,
		done:    make(chan struct{}),
	}
}

func (s *blockingWorkstationStream) Stdout() io.Reader { return s.stdoutR }

func (s *blockingWorkstationStream) Stderr() io.Reader { return s.stderrR }

func (s *blockingWorkstationStream) Wait() (int, error) {
	<-s.done
	return 137, errors.New("killed")
}

func (s *blockingWorkstationStream) Kill() error {
	s.killN.Add(1)
	s.once.Do(func() {
		_ = s.stdoutW.CloseWithError(context.Canceled)
		_ = s.stderrW.CloseWithError(context.Canceled)
		close(s.done)
	})
	return nil
}

func TestStreamAndCollectTimeoutKillsBlockedReaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	stream := newBlockingWorkstationStream()
	tool := &WorkstationExecTool{}
	ws := &store.Workstation{
		ID:       uuid.New(),
		TenantID: uuid.New(),
	}

	done := make(chan *Result, 1)
	go func() {
		done <- tool.streamAndCollect(ctx, stream, ws, uuid.NewString(), "session-timeout", "sleep 60")
	}()

	select {
	case result := <-done:
		if !result.IsError {
			t.Fatalf("expected timeout result to be an error, got %#v", result)
		}
		if stream.killN.Load() == 0 {
			t.Fatal("expected timed-out stream to be killed")
		}
	case <-time.After(time.Second):
		t.Fatal("streamAndCollect did not return after context timeout")
	}
}

func TestWorkstationExecRejectsShellCommandStringBeforeResolution(t *testing.T) {
	tool := NewWorkstationExecTool(nil, nil, nil, nil)
	result := tool.Execute(context.Background(), map[string]any{
		"command": "curl -s https://example.com 2>&1 || echo unavailable",
	})
	if !result.IsError {
		t.Fatal("expected shell command string to be rejected")
	}
	if !strings.Contains(result.ForLLM, "single executable") || !strings.Contains(result.ForLLM, "args") {
		t.Fatalf("expected executable/args guidance, got %q", result.ForLLM)
	}
}

func TestWorkstationExecRequiresContactGrantBeforePermissionCheck(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	contactID := uuid.New()
	workstationID := uuid.New()
	ws := &store.Workstation{
		ID:             workstationID,
		TenantID:       tenantID,
		WorkstationKey: "restricted",
		Active:         true,
	}
	links := &workstationResolverLinkStore{links: []store.AgentWorkstationLink{{
		AgentID: agentID, WorkstationID: workstationID, TenantID: tenantID,
	}}}
	baseCtx := store.WithAgentID(store.WithTenantID(context.Background(), tenantID), agentID)
	args := map[string]any{"workstation_id": workstationID.String(), "command": "echo"}

	t.Run("missing Contact identity", func(t *testing.T) {
		contactGrants := &workstationResolverContactGrantStore{allowed: true}
		tool := NewWorkstationExecTool(&workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}}, links, nil, nil)
		tool.SetContactGrantStore(contactGrants)
		permCalls := 0
		tool.SetPermCheck(func(context.Context, *store.Workstation, string, []string, map[string]string) error {
			permCalls++
			return nil
		})
		result := tool.Execute(baseCtx, args)
		if !result.IsError || permCalls != 0 || contactGrants.calls != 0 {
			t.Fatalf("result=%#v permCalls=%d contactGrantCalls=%d; want fail before lookups", result, permCalls, contactGrants.calls)
		}
	})

	t.Run("grant missing", func(t *testing.T) {
		contactGrants := &workstationResolverContactGrantStore{}
		tool := NewWorkstationExecTool(&workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}}, links, nil, nil)
		tool.SetContactGrantStore(contactGrants)
		permCalls := 0
		tool.SetPermCheck(func(context.Context, *store.Workstation, string, []string, map[string]string) error {
			permCalls++
			return nil
		})
		ctx := store.WithWorkstationContactID(baseCtx, contactID)
		result := tool.Execute(ctx, args)
		if !result.IsError || permCalls != 0 || contactGrants.calls != 1 {
			t.Fatalf("result=%#v permCalls=%d contactGrantCalls=%d; want Contact denial", result, permCalls, contactGrants.calls)
		}
	})

	t.Run("grant lookup failure", func(t *testing.T) {
		contactGrants := &workstationResolverContactGrantStore{err: errors.New("database unavailable")}
		tool := NewWorkstationExecTool(&workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}}, links, nil, nil)
		tool.SetContactGrantStore(contactGrants)
		permCalls := 0
		tool.SetPermCheck(func(context.Context, *store.Workstation, string, []string, map[string]string) error {
			permCalls++
			return nil
		})
		result := tool.Execute(store.WithWorkstationContactID(baseCtx, contactID), args)
		if !result.IsError || permCalls != 0 {
			t.Fatalf("result=%#v permCalls=%d; want fail closed", result, permCalls)
		}
	})

	t.Run("grant found progresses to exact permission error", func(t *testing.T) {
		contactGrants := &workstationResolverContactGrantStore{allowed: true}
		tool := NewWorkstationExecTool(&workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}}, links, nil, nil)
		tool.SetContactGrantStore(contactGrants)
		permissionErr := errors.New("binary curl is not in the workstation allowlist")
		permCalls := 0
		tool.SetPermCheck(func(context.Context, *store.Workstation, string, []string, map[string]string) error {
			permCalls++
			return permissionErr
		})
		result := tool.Execute(store.WithWorkstationContactID(baseCtx, contactID), args)
		if !result.IsError || result.ForLLM != permissionErr.Error() {
			t.Fatalf("result=%#v; want exact permission error", result)
		}
		if permCalls != 1 || contactGrants.calls != 1 || contactGrants.lastContact != contactID {
			t.Fatalf("permCalls=%d contactGrantCalls=%d lastContact=%s", permCalls, contactGrants.calls, contactGrants.lastContact)
		}
	})

	t.Run("exact Contact remains independent from shared credential identity", func(t *testing.T) {
		contactGrants := &workstationResolverContactGrantStore{allowed: true}
		tool := NewWorkstationExecTool(&workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}}, links, nil, nil)
		tool.SetContactGrantStore(contactGrants)
		permissionErr := errors.New("stop after policy check")
		tool.SetPermCheck(func(context.Context, *store.Workstation, string, []string, map[string]string) error {
			return permissionErr
		})
		ctx := store.WithCredentialUserID(baseCtx, "shared-group-user")
		ctx = store.WithWorkstationContactID(ctx, contactID)
		result := tool.Execute(ctx, args)
		if !result.IsError || result.ForLLM != permissionErr.Error() {
			t.Fatalf("result=%#v; want permission error", result)
		}
		if contactGrants.lastContact != contactID {
			t.Fatalf("grant checked Contact %s, want %s", contactGrants.lastContact, contactID)
		}
	})
}

func TestResolveWorkstationRequiresGrantForExplicitTarget(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	ws := &store.Workstation{
		ID:             workstationID,
		TenantID:       tenantID,
		WorkstationKey: "production",
		Active:         true,
	}
	tool := &WorkstationExecTool{
		wsStore:   &workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}},
		linkStore: &workstationResolverLinkStore{},
	}
	ctx := store.WithTenantID(context.Background(), tenantID)

	for _, target := range []string{workstationID.String(), ws.WorkstationKey} {
		t.Run(target, func(t *testing.T) {
			if got, err := tool.resolveWorkstation(ctx, map[string]any{"workstation_id": target}, agentID); err == nil || got != nil {
				t.Fatalf("resolveWorkstation(%q) = %#v, %v; want access denied", target, got, err)
			}
		})
	}
}

func TestResolveWorkstationAllowsAssignedExplicitTarget(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	ws := &store.Workstation{
		ID:             workstationID,
		TenantID:       tenantID,
		WorkstationKey: "development",
		Active:         true,
	}
	tool := &WorkstationExecTool{
		wsStore: &workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}},
		linkStore: &workstationResolverLinkStore{links: []store.AgentWorkstationLink{{
			AgentID: agentID, WorkstationID: workstationID, TenantID: tenantID,
		}}},
	}
	ctx := store.WithTenantID(context.Background(), tenantID)

	got, err := tool.resolveWorkstation(ctx, map[string]any{"workstation_id": workstationID.String()}, agentID)
	if err != nil {
		t.Fatalf("resolveWorkstation: %v", err)
	}
	if got.ID != workstationID {
		t.Fatalf("resolved workstation = %s, want %s", got.ID, workstationID)
	}
}

func TestResolveWorkstationRevocationTakesEffectImmediately(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	ws := &store.Workstation{ID: workstationID, TenantID: tenantID, WorkstationKey: "revocable", Active: true}
	links := &workstationResolverLinkStore{links: []store.AgentWorkstationLink{{
		AgentID: agentID, WorkstationID: workstationID, TenantID: tenantID,
	}}}
	tool := &WorkstationExecTool{
		wsStore:   &workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}},
		linkStore: links,
	}
	ctx := store.WithTenantID(context.Background(), tenantID)
	args := map[string]any{"workstation_id": workstationID.String()}

	if _, err := tool.resolveWorkstation(ctx, args, agentID); err != nil {
		t.Fatalf("resolve before revoke: %v", err)
	}
	if err := links.Unlink(ctx, agentID, workstationID); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if got, err := tool.resolveWorkstation(ctx, args, agentID); err == nil || got != nil {
		t.Fatalf("resolve after revoke = %#v, %v; want access denied", got, err)
	}
}

func TestResolveWorkstationFailsClosedWhenGrantLookupFails(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	ws := &store.Workstation{ID: workstationID, TenantID: tenantID, WorkstationKey: "fail-closed", Active: true}
	tool := &WorkstationExecTool{
		wsStore: &workstationResolverStore{items: map[uuid.UUID]*store.Workstation{workstationID: ws}},
		linkStore: &workstationResolverLinkStore{
			err: errors.New("database unavailable"),
		},
	}
	ctx := store.WithTenantID(context.Background(), tenantID)

	for name, args := range map[string]map[string]any{
		"explicit": {"workstation_id": workstationID.String()},
		"implicit": nil,
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := tool.resolveWorkstation(ctx, args, agentID); err == nil || got != nil {
				t.Fatalf("resolve on grant lookup error = %#v, %v; want denied", got, err)
			}
		})
	}
}

func TestResolveWorkstationUsesSoleOrDefaultGrant(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	items := map[uuid.UUID]*store.Workstation{
		firstID:  {ID: firstID, TenantID: tenantID, WorkstationKey: "first", Active: true},
		secondID: {ID: secondID, TenantID: tenantID, WorkstationKey: "second", Active: true},
	}
	ctx := store.WithTenantID(context.Background(), tenantID)

	t.Run("sole grant", func(t *testing.T) {
		tool := &WorkstationExecTool{
			wsStore: &workstationResolverStore{items: items},
			linkStore: &workstationResolverLinkStore{links: []store.AgentWorkstationLink{{
				AgentID: agentID, WorkstationID: firstID, TenantID: tenantID,
			}}},
		}
		got, err := tool.resolveWorkstation(ctx, nil, agentID)
		if err != nil || got.ID != firstID {
			t.Fatalf("resolve sole grant = %#v, %v; want %s", got, err, firstID)
		}
	})

	t.Run("default among multiple", func(t *testing.T) {
		tool := &WorkstationExecTool{
			wsStore: &workstationResolverStore{items: items},
			linkStore: &workstationResolverLinkStore{links: []store.AgentWorkstationLink{
				{AgentID: agentID, WorkstationID: firstID, TenantID: tenantID},
				{AgentID: agentID, WorkstationID: secondID, TenantID: tenantID, IsDefault: true},
			}},
		}
		got, err := tool.resolveWorkstation(ctx, nil, agentID)
		if err != nil || got.ID != secondID {
			t.Fatalf("resolve default grant = %#v, %v; want %s", got, err, secondID)
		}
	})

	t.Run("multiple without default", func(t *testing.T) {
		tool := &WorkstationExecTool{
			wsStore: &workstationResolverStore{items: items},
			linkStore: &workstationResolverLinkStore{links: []store.AgentWorkstationLink{
				{AgentID: agentID, WorkstationID: firstID, TenantID: tenantID},
				{AgentID: agentID, WorkstationID: secondID, TenantID: tenantID},
			}},
		}
		if got, err := tool.resolveWorkstation(ctx, nil, agentID); err == nil || got != nil {
			t.Fatalf("resolve multiple grants = %#v, %v; want explicit target error", got, err)
		}
	})
}
