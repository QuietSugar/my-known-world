package api

import (
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestSnapshotCache(t *testing.T) {
	cache := NewSnapshotCache(50 * time.Millisecond)

	// Initially nil
	if cache.Get() != nil {
		t.Fatal("expected nil cache initially")
	}

	g1 := graph.New()
	_ = g1.AddNode(graph.Node{ID: "node-1", Type: graph.Machine, Name: "M1", ModelVersion: 1})
	cache.Set(g1)

	// Hit
	hit := cache.Get()
	if hit == nil || len(hit.Nodes()) != 1 {
		t.Fatal("expected cached graph with 1 node")
	}

	// Invalidate
	cache.Invalidate()
	if cache.Get() != nil {
		t.Fatal("expected nil after invalidate")
	}

	// TTL Expiration
	cache.Set(g1)
	time.Sleep(60 * time.Millisecond)
	if cache.Get() != nil {
		t.Fatal("expected nil after TTL expiry")
	}
}
