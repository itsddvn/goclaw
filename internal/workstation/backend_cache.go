package workstation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// cachedBackend holds a Backend with its last-used timestamp.
type cachedBackend struct {
	backend  Backend
	lastUsed time.Time
	version  time.Time
}

// BackendCache is a TTL-based in-memory cache of Backend instances keyed by workstation UUID.
// On cache miss it opens a new Backend via the registered factory (workstation.Open).
// Invalidate(id) must be called on workstation update/delete to evict stale entries.
// sync.Mutex (not RWMutex) is used because lastUsed is mutated on every read-path hit,
// making an RWMutex unsafe — writes under RLock cause a data race.
type BackendCache struct {
	wsStore store.WorkstationStore
	cache   map[uuid.UUID]*cachedBackend
	// generations prevents a backend opened from an old workstation snapshot
	// from being inserted after an update/delete invalidation.
	generations      map[uuid.UUID]uint64
	globalGeneration uint64
	ttl              time.Duration
	mu               sync.Mutex
	open             BackendFactory
}

// NewBackendCache creates a BackendCache with the given TTL.
// A TTL of 10 minutes is recommended for production use.
func NewBackendCache(wsStore store.WorkstationStore, ttl time.Duration) *BackendCache {
	return &BackendCache{
		wsStore:     wsStore,
		cache:       make(map[uuid.UUID]*cachedBackend),
		generations: make(map[uuid.UUID]uint64),
		ttl:         ttl,
		open:        Open,
	}
}

// Get returns a cached Backend for wsID, or opens a new one via Open() on miss.
// Thread-safe. Uses a full Mutex (not RWMutex) because lastUsed is updated on cache hit,
// and mutating a field under RLock is a data race.
func (c *BackendCache) Get(ctx context.Context, wsID uuid.UUID) (Backend, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		c.mu.Lock()
		generation := c.generations[wsID]
		globalGeneration := c.globalGeneration
		c.mu.Unlock()

		// Always resolve the current row before using a cached connection. This
		// makes active=false and out-of-band connection edits fail closed even if
		// an asynchronous lifecycle event is delayed or dropped.
		ws, err := c.wsStore.GetByID(ctx, wsID)
		if err != nil {
			c.Invalidate(wsID)
			return nil, fmt.Errorf("workstation lookup: %w", err)
		}
		if !ws.Active {
			c.Invalidate(wsID)
			return nil, fmt.Errorf("workstation inactive: %s", wsID)
		}

		var stale Backend
		c.mu.Lock()
		if c.generations[wsID] != generation || c.globalGeneration != globalGeneration {
			c.mu.Unlock()
			continue
		}
		if cb, ok := c.cache[wsID]; ok {
			if time.Since(cb.lastUsed) < c.ttl && cb.version.Equal(ws.UpdatedAt) {
				cb.lastUsed = time.Now()
				backend := cb.backend
				c.mu.Unlock()
				return backend, nil
			}
			stale = cb.backend
			delete(c.cache, wsID)
		}
		c.mu.Unlock()
		if stale != nil {
			_ = stale.Close()
		}

		opener := c.open
		if opener == nil {
			opener = Open
		}
		backend, err := opener(ws)
		if err != nil {
			return nil, err
		}

		var displaced Backend
		c.mu.Lock()
		if c.generations[wsID] != generation || c.globalGeneration != globalGeneration {
			c.mu.Unlock()
			_ = backend.Close()
			continue
		}
		if cb, ok := c.cache[wsID]; ok {
			if time.Since(cb.lastUsed) < c.ttl && cb.version.Equal(ws.UpdatedAt) {
				existing := cb.backend
				c.mu.Unlock()
				_ = backend.Close()
				return existing, nil
			}
			displaced = cb.backend
		}
		c.cache[wsID] = &cachedBackend{backend: backend, lastUsed: time.Now(), version: ws.UpdatedAt}
		c.mu.Unlock()
		if displaced != nil {
			_ = displaced.Close()
		}
		return backend, nil
	}
}

// Invalidate evicts the cache entry for wsID.
// Should be called when a workstation is updated or deleted.
func (c *BackendCache) Invalidate(wsID uuid.UUID) {
	c.mu.Lock()
	if c.generations == nil {
		c.generations = make(map[uuid.UUID]uint64)
	}
	c.generations[wsID]++
	cb := c.cache[wsID]
	delete(c.cache, wsID)
	c.mu.Unlock()
	if cb != nil {
		_ = cb.backend.Close()
	}
}

// InvalidateAll clears the entire cache.
func (c *BackendCache) InvalidateAll() {
	c.mu.Lock()
	c.globalGeneration++
	backends := make([]Backend, 0, len(c.cache))
	for _, cb := range c.cache {
		backends = append(backends, cb.backend)
	}
	c.cache = make(map[uuid.UUID]*cachedBackend)
	c.mu.Unlock()
	for _, backend := range backends {
		_ = backend.Close()
	}
}
