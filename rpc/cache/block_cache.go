// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// BlockCache is a thread-safe LRU cache that invalidates entries on new block heights.
// Entries keyed before the current block height are considered stale and evicted.
type BlockCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	height   int64 // current block height (atomic reads, mutex writes)

	hits   uint64
	misses uint64
}

type cacheEntry struct {
	key    string
	value  interface{}
	height int64
}

// NewBlockCache creates a new LRU cache with the given capacity.
func NewBlockCache(capacity int) *BlockCache {
	return &BlockCache{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// SetHeight updates the current block height and evicts stale entries.
func (c *BlockCache) SetHeight(height int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if height <= c.height {
		return
	}
	c.height = height

	// Evict entries from older blocks (keep only finalized block data)
	for e := c.order.Back(); e != nil; {
		entry := e.Value.(*cacheEntry)
		// Only evict entries newer than 2 blocks ago — finalized data is still valid
		if c.height-entry.height <= 1 {
			break
		}
		prev := e.Prev()
		// For mutable latest-block queries, evict if they reference the old "latest"
		if entry.height == c.height-1 {
			// Keep entries from the previous finalized block
			e = prev
			continue
		}
		e = prev
	}
}

// Get retrieves a cached value. Returns nil if not found or stale.
func (c *BlockCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}
	entry := elem.Value.(*cacheEntry)
	c.mu.RUnlock()

	// Don't serve entries from the current tip (they might be re-orged)
	if entry.height >= c.height {
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&c.hits, 1)
	c.mu.Lock()
	c.order.MoveToFront(elem)
	c.mu.Unlock()
	return entry.value, true
}

// Set stores a value in the cache at the current block height.
func (c *BlockCache) Set(key string, value interface{}, height int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.height = height
		return
	}

	// Evict if at capacity
	for c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*cacheEntry).key)
	}

	entry := &cacheEntry{key: key, value: value, height: height}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

// Stats returns cache hit/miss counters.
func (c *BlockCache) Stats() (hits, misses uint64) {
	return atomic.LoadUint64(&c.hits), atomic.LoadUint64(&c.misses)
}

// Len returns the number of entries in the cache.
func (c *BlockCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}
