// Package cache provides Redis cache implementation.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements the Provider interface using Redis
type redisCache struct {
	client     *redis.Client
	namespace  string
	defaultTTL time.Duration
}

// RedisOptions contains Redis-specific configuration
type RedisOptions struct {
	Address      string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Namespace    string
	DefaultTTL   time.Duration
}

// DefaultRedisOptions returns sensible defaults
func DefaultRedisOptions() RedisOptions {
	return RedisOptions{
		Address:      "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		Namespace:    "ans:",
		DefaultTTL:   5 * time.Minute,
	}
}

// NewRedisCache creates a new Redis cache provider
func NewRedisCache(opts RedisOptions) (Provider, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         opts.Address,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	namespace := opts.Namespace
	if namespace == "" {
		namespace = "ans:"
	}

	return &redisCache{
		client:     client,
		namespace:  namespace,
		defaultTTL: opts.DefaultTTL,
	}, nil
}

func (c *redisCache) key(k string) string {
	return c.namespace + k
}

// Get retrieves a cached value by key
func (c *redisCache) Get(ctx context.Context, key string) (any, error) {
	data, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return value, nil
}

// Set stores a value with the specified TTL
func (c *redisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := c.client.Set(ctx, c.key(key), data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}

	return nil
}

// Delete removes a cached record
func (c *redisCache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, c.key(key)).Err(); err != nil {
		return fmt.Errorf("redis del error: %w", err)
	}
	return nil
}

// Exists checks if a key exists
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, c.key(key)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists error: %w", err)
	}
	return n > 0, nil
}

// Clear removes all entries with the namespace prefix
func (c *redisCache) Clear(ctx context.Context) error {
	// Use SCAN to find all keys with our prefix
	var cursor uint64
	var keys []string

	for {
		var batch []string
		var err error
		batch, cursor, err = c.client.Scan(ctx, cursor, c.namespace+"*", 100).Result()
		if err != nil {
			return fmt.Errorf("redis scan error: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("redis del error: %w", err)
		}
	}

	return nil
}

// Stats returns cache statistics
func (c *redisCache) Stats(ctx context.Context) (*Stats, error) {
	info, err := c.client.Info(ctx, "stats", "memory").Result()
	if err != nil {
		return nil, fmt.Errorf("redis info error: %w", err)
	}

	// Parse basic stats from info string
	// This is simplified - full implementation would parse all fields
	dbSize, _ := c.client.DBSize(ctx).Result()

	stats := &Stats{
		Size:    dbSize,
		MaxSize: -1, // Redis doesn't have a hard max
	}

	// Try to extract hit/miss stats from info
	_ = info // Would parse keyspace_hits, keyspace_misses

	return stats, nil
}

// Close releases resources
func (c *redisCache) Close() error {
	return c.client.Close()
}

// Name returns the provider name
func (c *redisCache) Name() string {
	return "redis"
}

func init() {
	Register("redis", func(opts Options, config map[string]interface{}) (Provider, error) {
		redisOpts := DefaultRedisOptions()
		redisOpts.DefaultTTL = opts.DefaultTTL
		redisOpts.Namespace = opts.Namespace

		// Read Redis-specific config from the config map
		if config != nil {
			if v, ok := config["address"].(string); ok {
				redisOpts.Address = v
			}
			if v, ok := config["password"].(string); ok {
				redisOpts.Password = v
			}
			if v, ok := config["db"].(int); ok {
				redisOpts.DB = v
			}
			if v, ok := config["poolSize"].(int); ok {
				redisOpts.PoolSize = v
			}
			if v, ok := config["minIdleConns"].(int); ok {
				redisOpts.MinIdleConns = v
			}
			if v, ok := config["dialTimeout"].(time.Duration); ok {
				redisOpts.DialTimeout = v
			}
			if v, ok := config["readTimeout"].(time.Duration); ok {
				redisOpts.ReadTimeout = v
			}
			if v, ok := config["writeTimeout"].(time.Duration); ok {
				redisOpts.WriteTimeout = v
			}
		}

		return NewRedisCache(redisOpts)
	})
}
