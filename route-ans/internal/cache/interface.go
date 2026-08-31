// Package cache provides pluggable caching implementations for the resolution server.
package cache

import (
	"context"
	"time"
)

// Provider defines the interface for cache implementations.
// Implementations must be safe for concurrent use.
type Provider interface {
	// Get retrieves a cached value by key.
	// Returns nil, nil if the key is not found (cache miss).
	Get(ctx context.Context, key string) (any, error)

	// Set stores a value with the specified TTL.
	// If TTL is 0, the default TTL should be used.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes a cached entry by key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all entries from the cache.
	Clear(ctx context.Context) error

	// Stats returns cache statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Close releases any resources held by the cache.
	Close() error

	// Name returns the provider name for logging/metrics.
	Name() string
}

// Stats contains cache statistics
type Stats struct {
	// Hits is the number of cache hits
	Hits int64

	// Misses is the number of cache misses
	Misses int64

	// Size is the current number of entries
	Size int64

	// MaxSize is the maximum capacity
	MaxSize int64

	// Evictions is the number of entries evicted
	Evictions int64
}

// HitRate calculates the cache hit rate (0.0 to 1.0)
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// Options contains common cache configuration options
type Options struct {
	// DefaultTTL is the default time-to-live for cached entries
	DefaultTTL time.Duration

	// MaxSize is the maximum number of entries (for memory-based caches)
	MaxSize int

	// CleanupInterval is how often expired entries are cleaned up
	CleanupInterval time.Duration

	// Namespace is an optional prefix for cache keys
	Namespace string
}

// DefaultOptions returns sensible default cache options
func DefaultOptions() Options {
	return Options{
		DefaultTTL:      5 * time.Minute,
		MaxSize:         10000,
		CleanupInterval: 1 * time.Minute,
		Namespace:       "ans:",
	}
}

// Factory is a function that creates a new cache provider
type Factory func(opts Options, config map[string]interface{}) (Provider, error)

// registry holds registered cache provider factories
var registry = make(map[string]Factory)

// Register registers a cache provider factory
func Register(name string, factory Factory) {
	registry[name] = factory
}

// New creates a new cache provider by name with optional provider-specific config
func New(name string, opts Options, config map[string]interface{}) (Provider, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, &ErrUnknownProvider{Name: name}
	}
	return factory(opts, config)
}

// Available returns the names of all registered cache providers
func Available() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// ErrUnknownProvider is returned when an unknown cache provider is requested
type ErrUnknownProvider struct {
	Name string
}

func (e *ErrUnknownProvider) Error() string {
	return "unknown cache provider: " + e.Name
}
