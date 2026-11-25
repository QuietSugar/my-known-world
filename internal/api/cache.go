package api

import (
	"sync"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// SnapshotCache provides a thread-safe in-memory cache for graph snapshots.
type SnapshotCache struct {
	mu       sync.RWMutex
	snapshot *graph.Graph
	cachedAt time.Time
	ttl      time.Duration
}

// NewSnapshotCache creates a snapshot cache with the specified time-to-live.
func NewSnapshotCache(ttl time.Duration) *SnapshotCache {
	return &SnapshotCache{
		ttl: ttl,
	}
}

// Get returns the cached graph snapshot if available and not expired.
func (c *SnapshotCache) Get() *graph.Graph {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.snapshot != nil && time.Since(c.cachedAt) < c.ttl {
		return c.snapshot
	}
	return nil
}

// Set saves a graph snapshot to the cache.
func (c *SnapshotCache) Set(g *graph.Graph) {
	if c == nil || g == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot = g
	c.cachedAt = time.Now()
}

// Invalidate explicitly discards any cached graph snapshot.
func (c *SnapshotCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot = nil
}
