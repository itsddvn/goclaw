package workstation

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type backendCacheWorkstationStore struct {
	store.WorkstationStore
	mu           sync.Mutex
	workstation  *store.Workstation
	calls        int
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

func (s *backendCacheWorkstationStore) GetByID(_ context.Context, id uuid.UUID) (*store.Workstation, error) {
	s.mu.Lock()
	s.calls++
	first := s.calls == 1
	copy := *s.workstation
	s.mu.Unlock()
	if first && s.firstStarted != nil {
		close(s.firstStarted)
		<-s.releaseFirst
	}
	return &copy, nil
}

func (s *backendCacheWorkstationStore) setWorkstation(ws *store.Workstation) {
	s.mu.Lock()
	s.workstation = ws
	s.mu.Unlock()
}

type cacheCloseBackend struct {
	closed atomic.Int64
}

func (b *cacheCloseBackend) Name() string                                         { return "test" }
func (b *cacheCloseBackend) HealthCheck(context.Context) error                    { return nil }
func (b *cacheCloseBackend) OpenSession(context.Context, string) (Session, error) { return nil, nil }
func (b *cacheCloseBackend) CloseSession(context.Context, string) error           { return nil }
func (b *cacheCloseBackend) Close() error {
	b.closed.Add(1)
	return nil
}

func TestBackendCacheInvalidateClosesBackend(t *testing.T) {
	id := uuid.New()
	backend := &cacheCloseBackend{}
	cache := &BackendCache{
		cache: map[uuid.UUID]*cachedBackend{id: {backend: backend, lastUsed: time.Now()}},
	}

	cache.Invalidate(id)
	if got := backend.closed.Load(); got != 1 {
		t.Fatalf("Close calls = %d, want 1", got)
	}
	if len(cache.cache) != 0 {
		t.Fatalf("cache size = %d, want 0", len(cache.cache))
	}
}

func TestBackendCacheInvalidateAllClosesBackends(t *testing.T) {
	first := &cacheCloseBackend{}
	second := &cacheCloseBackend{}
	cache := &BackendCache{cache: map[uuid.UUID]*cachedBackend{
		uuid.New(): {backend: first, lastUsed: time.Now()},
		uuid.New(): {backend: second, lastUsed: time.Now()},
	}}

	cache.InvalidateAll()
	if first.closed.Load() != 1 || second.closed.Load() != 1 {
		t.Fatalf("Close calls = (%d, %d), want (1, 1)", first.closed.Load(), second.closed.Load())
	}
	if len(cache.cache) != 0 {
		t.Fatalf("cache size = %d, want 0", len(cache.cache))
	}
}

func TestBackendCacheRejectsInactiveWorkstationBeforeCacheHit(t *testing.T) {
	id := uuid.New()
	backend := &cacheCloseBackend{}
	version := time.Now()
	wsStore := &backendCacheWorkstationStore{workstation: &store.Workstation{
		ID: id, Active: false, UpdatedAt: version,
	}}
	cache := NewBackendCache(wsStore, time.Minute)
	cache.cache[id] = &cachedBackend{backend: backend, lastUsed: time.Now(), version: version}

	if _, err := cache.Get(context.Background(), id); err == nil {
		t.Fatal("inactive workstation unexpectedly returned a cached backend")
	}
	if got := backend.closed.Load(); got != 1 {
		t.Fatalf("Close calls = %d, want 1", got)
	}
}

func TestBackendCacheInvalidationDiscardsInFlightStaleLoad(t *testing.T) {
	id := uuid.New()
	oldVersion := time.Now().Add(-time.Minute)
	newVersion := time.Now()
	wsStore := &backendCacheWorkstationStore{
		workstation:  &store.Workstation{ID: id, Name: "old", Active: true, UpdatedAt: oldVersion},
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	cache := NewBackendCache(wsStore, time.Minute)
	openedName := make(chan string, 2)
	cache.open = func(ws *store.Workstation) (Backend, error) {
		openedName <- ws.Name
		return &cacheCloseBackend{}, nil
	}

	result := make(chan error, 1)
	go func() {
		_, err := cache.Get(context.Background(), id)
		result <- err
	}()

	<-wsStore.firstStarted
	cache.Invalidate(id)
	wsStore.setWorkstation(&store.Workstation{ID: id, Name: "new", Active: true, UpdatedAt: newVersion})
	close(wsStore.releaseFirst)

	if err := <-result; err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if name := <-openedName; name != "new" {
		t.Fatalf("opened workstation = %q, want new", name)
	}
	select {
	case name := <-openedName:
		t.Fatalf("opened an extra stale backend for %q", name)
	default:
	}
}
