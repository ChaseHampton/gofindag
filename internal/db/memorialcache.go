package db

import (
	"context"
	"sync"

	"github.com/ChaseHampton/gofindag/internal/config"
	"github.com/ChaseHampton/gofindag/internal/search"
)

type MemorialCache struct {
	mu    sync.RWMutex
	cache map[int64]bool
	cfg   *config.Config
}

func NewMemorialCache() *MemorialCache {
	return &MemorialCache{
		cache: make(map[int64]bool),
	}
}

func (mc *MemorialCache) FilterIds(ids []int64) []int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var filtered []int64
	for _, id := range ids {
		if !mc.cache[id] {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

// returns both new and previously seen memorials
func (mc *MemorialCache) FilterMemorials(memorials []search.Memorial) ([]search.Memorial, []search.Memorial) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var new []search.Memorial
	var seen []search.Memorial
	for _, memorial := range memorials {
		if !mc.cache[memorial.MemorialID] {
			new = append(new, memorial)
		} else {
			seen = append(seen, memorial)
		}
	}
	return new, seen
}

func (mc *MemorialCache) MarkSeen(ids []int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, id := range ids {
		mc.cache[id] = true
	}
}

func (mc *MemorialCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.cache)
}

func (mc *MemorialCache) LoadIds(ctx context.Context, ids []int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, id := range ids {
		mc.cache[id] = true
	}
}
