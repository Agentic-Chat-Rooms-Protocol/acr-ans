# Cache Providers Reference

Complete reference for cache provider implementations.

## Overview

Cache providers store resolution results to improve performance. Supported:

- **Memory**: In-memory LRU cache
- **Redis**: Distributed Redis cache

## Interface Definition

```go
type Cache interface {
    // Get cached resolution result
    Get(ctx context.Context, key string) (*resolver.ResolutionResult, error)
    
    // Set resolution result with TTL
    Set(ctx context.Context, key string, value *resolver.ResolutionResult, ttl time.Duration) error
    
    // Delete cached entry
    Delete(ctx context.Context, key string) error
    
    // Clear all entries
    Clear(ctx context.Context) error
    
    // Get cache size in bytes
    Size() int64
}
```

## Memory Cache

### Configuration

```yaml
cache:
  type: memory
  ttl: 3600  # Default TTL in seconds
  memory:
    max_size_mb: 256
    max_entries: 10000
    eviction_policy: lru  # lru, lfu, fifo
```

### Features

- **LRU Eviction**: Least Recently Used
- **Size Limit**: Maximum memory usage
- **Entry Limit**: Maximum number of entries
- **Thread-Safe**: sync.Map implementation

### Implementation Details

```go
type MemoryCache struct {
    data       sync.Map
    maxSize    int64
    maxEntries int
    ttl        time.Duration
    size       atomic.Int64
    entries    atomic.Int64
}

func (m *MemoryCache) Get(ctx context.Context, key string) (*resolver.ResolutionResult, error) {
    value, ok := m.data.Load(key)
    if !ok {
        return nil, ErrCacheMiss
    }
    
    entry := value.(*cacheEntry)
    
    // Check expiration
    if time.Now().After(entry.expiresAt) {
        m.data.Delete(key)
        return nil, ErrCacheMiss
    }
    
    return entry.result, nil
}
```

### Cache Entry

```go
type cacheEntry struct {
    result    *resolver.ResolutionResult
    size      int64
    expiresAt time.Time
    accessed  time.Time
}
```

### Eviction Strategy

When cache is full:

1. Find LRU entry (oldest access time)
2. Delete entry
3. Update size/count
4. Insert new entry

### Metrics

```
ans_cache_hits_total{cache="memory"}
ans_cache_misses_total{cache="memory"}
ans_cache_size_bytes{cache="memory"}
ans_cache_entries_total{cache="memory"}
ans_cache_evictions_total{cache="memory"}
```

## Redis Cache

### Configuration

```yaml
cache:
  type: redis
  ttl: 3600
  redis:
    host: localhost
    port: 6379
    password: ${REDIS_PASSWORD}
    db: 0
    pool_size: 10
    max_retries: 3
    min_retry_backoff: 8ms
    max_retry_backoff: 512ms
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    pool_timeout: 4s
    idle_timeout: 5m
    prefix: "ans:"
```

### Environment Variables

```bash
export REDIS_HOST=redis
export REDIS_PORT=6379
export REDIS_PASSWORD=secret
```

### Key Format

```
ans:resolve:{protocol}:{capability}:{pid}:{version}:{fqdn}
```

Example:
```
ans:resolve:mcp:chatbot:PID-5678:1.2.3:example.com
```

### Features

- **Distributed**: Shared across multiple resolver instances
- **Persistence**: Optional persistence (AOF/RDB)
- **Clustering**: Redis Cluster support
- **Sentinel**: High availability with Redis Sentinel

### Implementation

```go
type RedisCache struct {
    client *redis.Client
    prefix string
    ttl    time.Duration
}

func (r *RedisCache) Get(ctx context.Context, key string) (*resolver.ResolutionResult, error) {
    data, err := r.client.Get(ctx, r.prefix+key).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    if err != nil {
        return nil, err
    }
    
    var result resolver.ResolutionResult
    err = json.Unmarshal(data, &result)
    return &result, err
}

func (r *RedisCache) Set(ctx context.Context, key string, value *resolver.ResolutionResult, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return r.client.Set(ctx, r.prefix+key, data, ttl).Err()
}
```

### Redis Cluster

```yaml
cache:
  type: redis-cluster
  redis_cluster:
    addrs:
      - redis-1:6379
      - redis-2:6379
      - redis-3:6379
    password: ${REDIS_PASSWORD}
    read_only: false
    route_by_latency: true
```

### Redis Sentinel

```yaml
cache:
  type: redis-sentinel
  redis_sentinel:
    master_name: mymaster
    sentinel_addrs:
      - sentinel-1:26379
      - sentinel-2:26379
      - sentinel-3:26379
    password: ${REDIS_PASSWORD}
```

### Metrics

```
ans_cache_hits_total{cache="redis"}
ans_cache_misses_total{cache="redis"}
ans_cache_operations_total{cache="redis",operation="get|set|delete"}
ans_cache_errors_total{cache="redis",error_type="timeout|connection"}
```

## Cache Strategies

### Time-to-Live (TTL)

```yaml
# Use agent record expiration
cache:
  ttl: 0  # Use expires_at from record
  
# Fixed TTL
cache:
  ttl: 3600  # 1 hour
  
# Min/Max TTL
cache:
  min_ttl: 60
  max_ttl: 86400
```

### Cache Warming

Pre-populate cache with frequently accessed agents:

```go
func WarmCache(cache cache.Cache, resolver resolver.Resolver, ansNames []string) error {
    for _, name := range ansNames {
        result, err := resolver.Resolve(context.Background(), name)
        if err != nil {
            continue
        }
        cache.Set(context.Background(), name, result, time.Hour)
    }
    return nil
}
```

### Cache Invalidation

```go
// Delete specific entry
cache.Delete(ctx, "ans:resolve:mcp:chatbot:PID-5678:1.2.3:example.com")

// Clear all
cache.Clear(ctx)

// Pattern-based (Redis only)
keys, _ := redisClient.Keys(ctx, "ans:resolve:mcp:*").Result()
for _, key := range keys {
    redisClient.Del(ctx, key)
}
```

## Multi-Level Caching (Future)

L1 (memory) + L2 (Redis):

```yaml
cache:
  type: multi
  levels:
    - type: memory
      max_size_mb: 64
      ttl: 300
    - type: redis
      host: redis
      ttl: 3600
```

## Performance Comparison

| Metric | Memory | Redis Local | Redis Remote |
|--------|--------|-------------|--------------|
| **Latency** | <1ms | 1-2ms | 5-10ms |
| **Throughput** | 1M+ ops/s | 100K ops/s | 10K ops/s |
| **Capacity** | Limited by RAM | Large | Very Large |
| **Distributed** | No | Yes | Yes |
| **Persistence** | No | Yes | Yes |

## Monitoring

### Cache Hit Rate

```promql
# Cache hit rate
rate(ans_cache_hits_total[5m]) / 
  (rate(ans_cache_hits_total[5m]) + rate(ans_cache_misses_total[5m]))
```

Target: >80% hit rate

### Cache Size

```promql
ans_cache_size_bytes / (256 * 1024 * 1024)  # Utilization
```

### Eviction Rate

```promql
rate(ans_cache_evictions_total[5m])
```

Should be low for optimal performance.

## Troubleshooting

### High Miss Rate

- Increase cache size
- Increase TTL
- Implement cache warming

### Memory Pressure

- Reduce max_size_mb
- Reduce max_entries
- Implement LRU eviction

### Redis Connection Issues

```bash
# Test connection
redis-cli -h redis -p 6379 ping

# Check latency
redis-cli -h redis -p 6379 --latency

# Monitor commands
redis-cli -h redis -p 6379 monitor
```

## Best Practices

1. **Enable Caching**: Always use cache in production
2. **Right Size**: Balance memory vs hit rate
3. **Monitor Metrics**: Track hit rate and evictions
4. **Use Redis**: For distributed deployments
5. **Cache Warming**: Pre-populate frequently accessed entries
6. **TTL Strategy**: Use agent expiration when available

## Next Steps

- **[Registry Adapters](registry-adapters.md)** - Registry implementations
- **[Trust Verification](trust-verification.md)** - Certificate verification
- **[Monitoring](../monitoring.md)** - Observability
