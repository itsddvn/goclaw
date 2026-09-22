package security

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type blockingAllowlistStore struct {
	store.WorkstationPermissionStore
	mu           sync.Mutex
	permissions  []store.WorkstationPermission
	calls        int
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

func (s *blockingAllowlistStore) ListForWorkstation(_ context.Context, _ uuid.UUID) ([]store.WorkstationPermission, error) {
	s.mu.Lock()
	s.calls++
	first := s.calls == 1
	snapshot := append([]store.WorkstationPermission(nil), s.permissions...)
	s.mu.Unlock()
	if first {
		close(s.firstStarted)
		<-s.releaseFirst
	}
	return snapshot, nil
}

func (s *blockingAllowlistStore) setPermissions(permissions []store.WorkstationPermission) {
	s.mu.Lock()
	s.permissions = append([]store.WorkstationPermission(nil), permissions...)
	s.mu.Unlock()
}

func TestValidateAllowedBinaryPattern(t *testing.T) {
	for _, pattern := range []string{"git", "python*", "node-20", "go1.26"} {
		if err := ValidateAllowedBinaryPattern(pattern); err != nil {
			t.Errorf("ValidateAllowedBinaryPattern(%q) = %v", pattern, err)
		}
	}
	for _, pattern := range []string{"", "*", "git *", "/usr/bin/git", "git|sh", "py*thon", " git"} {
		if err := ValidateAllowedBinaryPattern(pattern); err == nil {
			t.Errorf("ValidateAllowedBinaryPattern(%q) unexpectedly succeeded", pattern)
		}
	}
}

func TestValidateLauncherArgsDeniesEnvCommandLaunch(t *testing.T) {
	if reason := validateLauncherArgs("env", []string{"bash", "-lc", "id"}); reason == "" {
		t.Fatal("expected env with command args to be denied")
	}
}

func TestValidateLauncherArgsAllowsPlainNonLauncherCommand(t *testing.T) {
	if reason := validateLauncherArgs("git", []string{"status"}); reason != "" {
		t.Fatalf("expected git args to be allowed, got %q", reason)
	}
}

func TestAllowlistInvalidationDiscardsInFlightStaleLoad(t *testing.T) {
	workstationID := uuid.New()
	permissionStore := &blockingAllowlistStore{
		permissions:  []store.WorkstationPermission{{Pattern: "git", Enabled: true}},
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	checker := NewAllowlistChecker(permissionStore, time.Minute)
	ws := &store.Workstation{ID: workstationID}

	result := make(chan error, 1)
	go func() {
		result <- checker.Check(context.Background(), ws, "git", nil)
	}()

	<-permissionStore.firstStarted
	permissionStore.setPermissions(nil)
	checker.Invalidate(workstationID)
	close(permissionStore.releaseFirst)

	if err := <-result; err == nil {
		t.Fatal("stale allowlist load allowed a revoked command")
	}
	permissionStore.mu.Lock()
	calls := permissionStore.calls
	permissionStore.mu.Unlock()
	if calls != 2 {
		t.Fatalf("permission loads = %d, want 2 after generation changed", calls)
	}
}
