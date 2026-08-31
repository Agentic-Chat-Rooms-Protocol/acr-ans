// Package cache provides pluggable caching implementations for the resolution server.
package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// memoryCache is an in-memory LRU cache implementation
type memoryCache struct {
	mu            sync.RWMutex
	items         map[string]*cacheEntry
	lru           *list.List
	maxSize       int
	defaultTTL    time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	// Stats
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

// cacheEntry represents a cached item
type cacheEntry struct {
	key       string
	value     any
	expiresAt time.Time
	element   *list.Element
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(opts Options) (Provider, error) {
	if opts.MaxSize <= 0 {
		opts.MaxSize = 10000
	}
	if opts.DefaultTTL <= 0 {
		opts.DefaultTTL = 5 * time.Minute
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = 1 * time.Minute
	}

	c := &memoryCache{
		items:       make(map[string]*cacheEntry),
		lru:         list.New(),
		maxSize:     opts.MaxSize,
		defaultTTL:  opts.DefaultTTL,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup
	c.cleanupTicker = time.NewTicker(opts.CleanupInterval)
	go c.cleanupLoop()

	return c, nil
}

func (c *memoryCache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *memoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.items {
		if now.After(entry.expiresAt) {
			c.lru.Remove(entry.element)
			delete(c.items, key)
			c.evictions.Add(1)
		}
	}
}

// Get retrieves a cached value by key
func (c *memoryCache) Get(ctx context.Context, key string) (any, error) {
	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.misses.Add(1)
		return nil, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		c.misses.Add(1)
		// Lazy deletion - will be cleaned up by cleanup goroutine
		return nil, nil
	}

	// Move to front of LRU
	c.mu.Lock()
	c.lru.MoveToFront(entry.element)
	c.mu.Unlock()

	c.hits.Add(1)
	return entry.value, nil
}

// Set stores a value with the specified TTL
func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if existing, exists := c.items[key]; exists {
		// Update existing entry
		existing.value = value
		existing.expiresAt = time.Now().Add(ttl)
		c.lru.MoveToFront(existing.element)
		return nil
	}

	// Evict if at capacity
	for len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	// Create new entry
	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	entry.element = c.lru.PushFront(entry)
	c.items[key] = entry

	return nil
}

func (c *memoryCache) evictOldest() {
	oldest := c.lru.Back()
	if oldest == nil {
		return
	}

	entry := oldest.Value.(*cacheEntry)
	c.lru.Remove(oldest)
	delete(c.items, entry.key)
	c.evictions.Add(1)
}

// Delete removes a cached record
func (c *memoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.items[key]; exists {
		c.lru.Remove(entry.element)
		delete(c.items, key)
	}

	return nil
}

// Exists checks if a key exists
func (c *memoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear removes all entries
func (c *memoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheEntry)
	c.lru = list.New()

	return nil
}

// Stats returns cache statistics
func (c *memoryCache) Stats(ctx context.Context) (*Stats, error) {
	c.mu.RLock()
	size := int64(len(c.items))
	c.mu.RUnlock()

	return &Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Size:      size,
		MaxSize:   int64(c.maxSize),
		Evictions: c.evictions.Load(),
	}, nil
}

// Close releases resources
func (c *memoryCache) Close() error {
	c.cleanupTicker.Stop()
	close(c.stopCleanup)
	return nil
}

const providerName = "memory"

// Name returns the provider name
func (c *memoryCache) Name() string {
	return providerName
}

func init() {
	Register(providerName, func(opts Options, config map[string]interface{}) (Provider, error) {
		return NewMemoryCache(opts)
	})
}
