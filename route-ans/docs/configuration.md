# Route ANS

Complete API reference for Route ANS, an Agent Name System (ANS) resolution server.

!!! info "Implementation Status"
    Some configuration options are designed for future features. These are marked with badges:
    
    - 🟢 **Implemented** - Fully functional
    - 🟡 **Partial** - Basic implementation, limited functionality
    - 🔴 **Future** - Not yet implemented, reserved for future use
    - ⚠️ **Optional** - Can be disabled/omitted

## Quick Navigation

Jump to a specific section:

- **🟢 [Server Configuration](#server-server-configuration)** - HTTP server settings
- **🟢 [Cache Configuration](#cache-cache-configuration)** - Memory/Redis cache providers
- **🟢 [Queue Configuration](#queue-queue-configuration)** - Event processing for registry emitted events
- **🟢 [Registry Configuration](#registries-registry-adapters)** - GoDaddy and mock registries
- **🔴 [Trust/Verification](#trust-trust-and-verification)** - Security and verification settings (awaiting registry support)
- **🟢 [Rate Limiting](#ratelimit-rate-limiting)** - API rate limiting
- **🟢 [Telemetry](#telemetry-telemetry-configuration)** - Logging, metrics, tracing
- **🟢 [Environment Variables](#environment-variables)** - Variable substitution guide
- **🟢 [Example Configurations](#complete-example-configurations)** - Sample config files

---

## Overview

The resolver uses YAML configuration with environment variable substitution.

**Format:** `${VAR_NAME:default_value}`

**Example:**

```yaml
apiKey: "${GODADDY_API_KEY:your-key-here}"
```

## Configuration Structure

```yaml
server:
cache:
queue:
registries:
trust:
rateLimit:  
telemetry:
```

---

## Complete Configuration Reference

### `server` - Server Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Configures the HTTP server that handles incoming resolution requests.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `host` | string | 0.0.0.0 | Server bind address | 🟢 Required |
| `port` | int | 8080 | Server port | 🟢 Required |
| `readTimeout` | duration | 30s | Maximum duration for reading request | 🟢 |
| `writeTimeout` | duration | 30s | Maximum duration for writing response | 🟢 |
| `idleTimeout` | duration | 120s | Maximum idle connection duration | 🟢 |
| `maxHeaderBytes` | int | 1048576 | Maximum request header size (bytes) | 🟢 |
| `gracefulShutdownTimeout` | duration | 30s | Time to wait for connections to close during shutdown | 🟢 |

**Implementation Notes:**

- **RESTful JSON API** - Follows DNS-over-HTTPS (RFC 8484) precedent but adapted for ANS with rich JSON responses instead of DNS wire format
- **TLS via reverse proxy** - Delegate TLS/mTLS to battle-tested infrastructure (nginx, Envoy, Cloudflare, HAProxy) for better security, certificate management, and performance
- **HTTP/2 support** - Native multiplexing and header compression for reduced latency and improved throughput

**Example:**
```yaml
server:
  host: 0.0.0.0
  port: 8080
  readTimeout: 30s
  writeTimeout: 30s
  idleTimeout: 120s
  maxHeaderBytes: 1048576
  gracefulShutdownTimeout: 30s
```

---

### `server.http` - HTTP Server Settings

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Controls the HTTP server that exposes the REST API and Swagger UI.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `host` | string | 0.0.0.0 | Network interface to bind to (0.0.0.0 = all interfaces) | 🟢 |
| `port` | int | 8080 | TCP port to listen on | 🟢 |
| `readTimeout` | duration | 30s | Maximum time to read the entire request | 🟢 |
| `writeTimeout` | duration | 30s | Maximum time to write the response | 🟢 |
| `idleTimeout` | duration | 120s | Maximum time to wait for next request when keep-alive is enabled | 🟢 |
| `maxHeaderBytes` | int | 1048576 | Maximum size of request headers (1MB) | 🟢 |

**Performance Tips:**

- Increase `readTimeout`/`writeTimeout` if resolution takes >30s (e.g., slow DNS)
- Decrease `idleTimeout` to free connections faster under high load
- Increase `maxHeaderBytes` only if you need larger headers (rarely needed)

---

### `cache` - Cache Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Configures caching to speed up repeated resolution requests. Critical for performance.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `provider` | string | memory | Cache backend: `memory` (in-process) or `redis` (distributed) | 🟢 |
| `ttl` | TTLConfig | - | Time-to-live settings for different record types | 🟢 |
| `memory` | MemoryCacheConfig | - | In-memory cache settings (used when provider=memory) | 🟢 |
| `redis` | RedisCacheConfig | - | Redis cache settings (used when provider=redis) | 🟢 |

**Cache Strategy:**

- **Cache hit**: Sub-millisecond response time
- **Cache miss**: 100-500ms (registry lookup + verification)
- **Recommended**: Start with `memory`, upgrade to `redis` for multi-instance deployments

**Example:**
```yaml
cache:
  provider: memory
  ttl:
    default: 300s      # 5 minutes - general TTL
    verified: 600s     # 10 minutes - verified agents cached longer
    revoked: 3600s     # 1 hour - revoked agents cached to prevent repeated checks
    notFound: 60s      # 1 minute - not-found results cached briefly
  memory:
    maxSize: 10000
    cleanupInterval: 60s
```

**When to use Redis:**

- Running multiple resolver instances
- Need cache sharing across instances
- Handling >1000 requests/second
- Want persistent cache across restarts

---

### `cache.ttl` - Cache TTL Settings

🟢 **Status:** Fully Implemented  
**Type:** Object

Controls how long different types of records are cached before being re-fetched from the registry.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `default` | duration | 300s | Default TTL for records without specific status | 🟢 |
| `verified` | duration | 600s | TTL for successfully verified agent records | 🟢 |
| `revoked` | duration | 3600s | TTL for revoked agents (cached longer to avoid repeated lookups) | 🟢 |
| `notFound` | duration | 60s | TTL for "agent not found" results (short to catch new registrations) | 🟢 |

**TTL Strategy:**

- **Longer TTL** = Better performance, less registry load, potentially stale data
- **Shorter TTL** = More up-to-date, higher registry load, slower responses
- **Revoked agents** cached longest - revocation rarely changes
- **Not-found** cached shortest - agents might get registered

---

### `cache.redis` - Redis Cache Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Used when:** `cache.provider: redis`

Redis configuration for distributed caching across multiple resolver instances.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `address` | string | localhost:6379 | Redis server address (host:port) | 🟢 |
| `password` | string | "" | Redis authentication password | 🟢 |
| `db` | int | 0 | Redis database number (0-15) | 🟢 |
| `maxRetries` | int | 3 | Max retry attempts for failed operations | 🟢 |
| `poolSize` | int | 10 | Max connections in pool | 🟢 |
| `minIdleConns` | int | 5 | Min idle connections kept open | 🟢 |
| `dialTimeout` | duration | 5s | Timeout for establishing connection | 🟢 |
| `readTimeout` | duration | 3s | Timeout for read operations | 🟢 |
| `writeTimeout` | duration | 3s | Timeout for write operations | 🟢 |
| `poolTimeout` | duration | 4s | Timeout waiting for connection from pool | 🟢 |
| `idleTimeout` | duration | 5m | Timeout before closing idle connections | 🟢 |
| `keyPrefix` | string | ans:cache: | Prefix for all cache keys (prevents collisions) | 🟢 |

**Example:**
```yaml
cache:
  provider: redis
  redis:
    address: "${REDIS_ADDRESS:localhost:6379}"
    password: "${REDIS_PASSWORD:}"
    db: 0
    poolSize: 20        # Increase for high-traffic deployments
    keyPrefix: ans:cache:
```

**Performance Tuning:**

- Increase `poolSize` for higher concurrency
- Adjust timeouts based on network latency to Redis
- Use separate Redis instance from application data
- Monitor connection pool usage

---

### `queue` - Queue Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Required:** Yes

Message queue configuration for handling asynchronous registry events.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | **false** | Enable/disable queue event processing | 🟢 ⚠️ Disabled by default |
| `provider` | string | memory | Queue provider: `memory`, `redis-streams` | 🟢 |
| `bufferSize` | int | 1000 | Buffer size for in-memory queue (memory provider) | 🟢 |
| `redis-streams` | RedisStreamsConfig | - | Redis Streams settings (redis-streams provider) | 🟢 |

**What it does when enabled:**

1. **Poll registry events** - Periodically fetch events from registry event endpoint
2. **Cache invalidation** - Automatically invalidate cache when registry events arrive
3. **Event processing** - Handle registered, renewed, revoked, deprecated, expired events
4. **Cursor-based pagination** - Track cursor position to resume from last processed event
5. **Metrics tracking** - Record queue statistics (processed, failed, pending)

**Registry-Specific Implementation:**

*GoDaddy Registry:*

- Endpoint: `GET /v1/agents/events`
- Polling interval: 10 seconds
- Pagination: Cursor-based via `lastLogId` parameter
- Batch size: 100 events per request
- Event retention: 30 days
- Authentication: SSO-Key credentials

**Event Types Supported:**

- `registered` - New agent registered, invalidate cache
- `renewed` - Agent renewed, refresh cache entry
- `revoked` - Agent revoked, remove from cache immediately
- `deprecated` - Agent deprecated, remove from cache
- `expired` - Agent expired, remove from cache
- `updated` - Agent metadata updated, invalidate cache

*Note: Registry-specific event names (e.g., GoDaddy's `AGENT_REGISTERED`) are automatically mapped to internal event types.*

**Default Configuration (Disabled):**
```yaml
queue:
  enabled: false      # Enable to start polling registry events
  provider: memory
  bufferSize: 1000
```

**Enable for production:**
```yaml
queue:
  enabled: true       # Start polling registry events
  provider: memory    # Or redis-streams for distributed processing
  bufferSize: 1000
```

**Use cases:**

- Automatic cache invalidation when agents are updated/revoked
- Real-time synchronization with registry changes
- Metrics on registry event activity
- Multi-resolver deployments needing cache coordination

---

### `queue.redis-streams` - Redis Streams Configuration

🟢 **Status:** Fully Implemented  
**Type:** Object  
**Used when:** `queue.provider: redis-streams`

Redis Streams configuration for distributed event processing across multiple resolver instances.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `address` | string | localhost:6379 | Redis server address (host:port) | 🟢 |
| `password` | string | "" | Redis authentication password | 🟢 |
| `db` | int | 0 | Redis database number (0-15) | 🟢 |
| `stream` | string | ans:events | Redis stream name for events | 🟢 |
| `consumerGroup` | string | resolver-group | Consumer group name | 🟢 |
| `consumer` | string | resolver-1 | This consumer's unique name | 🟢 |
| `blockTimeout` | duration | 5s | Max time to block waiting for events | 🟢 |
| `batchSize` | int | 10 | Max events to fetch per batch | 🟢 |

**Example:**
```yaml
queue:
  enabled: true
  provider: redis-streams
  redis-streams:
    address: "${REDIS_ADDRESS:localhost:6379}"
    password: "${REDIS_PASSWORD:}"
    db: 0
    stream: ans:events
    consumerGroup: resolver-group
    consumer: resolver-1
    blockTimeout: 5s
    batchSize: 10
```

**How it works:**

1. **Redis Streams** - Uses native Redis Streams (XREADGROUP/XADD) for event distribution
2. **Consumer Groups** - Multiple resolvers join same consumer group for load balancing
3. **Acknowledgment** - Events are acknowledged after successful processing
4. **Pending tracking** - Failed events remain pending for retry
5. **Stats available** - Tracks processed, failed, pending, and consumer lag

**When to use Redis Streams:**

- Running multiple resolver instances
- Need distributed event processing across instances
- Want event persistence and replay capability
- Require guaranteed event delivery with acks
- Need metrics on consumer lag and pending events

**Performance tuning:**

- Increase `batchSize` for higher throughput (up to 100)
- Adjust `blockTimeout` based on event frequency
- Use separate Redis instance from cache for isolation
- Monitor consumer lag via Stats endpoint

---

### `store` - Data Store Configuration

### `registries` - Registry Adapters

🟢 **Status:** Fully Implemented (GoDaddy, Mock)  
**Type:** Array of RegistryConfig  
**Required:** Yes (at least one)

!!! success "Supported Registries"
    - ✅ **GoDaddy** - Production registry, fully functional
    - ✅ **Mock** - Testing/development registry with configurable responses

Configures connections to ANS registries where agents are registered. The resolver supports multiple registries with priority-based fallback.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `name` | string | - | Human-readable registry identifier | 🟢 |
| `type` | string | - | Registry type: `godaddy` (working), `mock` (working) | 🟢 |
| `enabled` | bool | true | Enable/disable this registry | 🟢 |
| `priority` | int | 1 | Priority when multiple registries configured (lower = higher) | 🟢 |
| `timeout` | duration | 10s | Max time for registry API calls | 🟢 |
| `retries` | int | 3 | Max retry attempts on transient failures | 🟢 |
| `retryBackoff` | duration | 1s | Initial backoff duration (exponential backoff used) | 🟢 |
| `config` | map | - | Registry-specific configuration (see below) | 🟢 |

**Multi-Registry Support:**
You can configure multiple registries with different priorities. The resolver will try them in priority order until one succeeds.

**Example:**
```yaml
registries:
  - name: godaddy-primary
    type: godaddy
    enabled: true
    priority: 1
    timeout: 10s
    retries: 3
    retryBackoff: 1s
    config:
      baseURL: "https://api.godaddy.com/v1"
      apiKey: "${GODADDY_API_KEY}"
      apiSecret: "${GODADDY_API_SECRET}"
```

### `registries[].config` - GoDaddy Registry

**Type:** Object  
**When:** `type: godaddy`

| Field | Type | Required | Description | Status |
|-------|------|----------|-------------|--------|
| `baseURL` | string | Yes | GoDaddy API base URL | 🟢 |
| `apiKey` | string | Yes | API key from developer portal | 🟢 |
| `apiSecret` | string | Yes | API secret | 🟢 |

**URLs:**

- Production: `https://api.godaddy.com/v1`
- OTE (test): `https://api.ote-godaddy.com/v1`

**Get credentials:** https://developer.godaddy.com/keys

---

### `trust` - Trust and Verification

🔴 **Status:** Future - Awaiting Registry Support  
**Type:** Object  
**Required:** Yes

!!! info "Registry Support Required"
    Trust verification code is implemented but **cannot be used** because registries must include certificate fingerprints in resolution responses. Currently, no supported registries provide this data. Will be available once registry implementations are updated.

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `provider` | string | file | Trust provider: `file`, `vault`, `k8s-secret` | 🟢 |
| `verification` | VerificationConfig | - | Verification settings | 🟢 |
| `file` | FileTrustConfig | - | File-based trust settings | 🟢 |

**Example:**
```yaml
trust:
  provider: file
  verification:
    enabled: false             # Disabled: awaiting registry support for fingerprints
    requireSignature: false    # GoDaddy doesn't sign responses yet
    requireMerkleProof: false  # Future feature
    checkRevocation: false     # Disabled by default
```

---

### `trust.verification` - Verification Settings

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | **false** | Enable trust verification (see note above) | 🔴 Future |
| `requireSignature` | bool | false | Require registry signature on responses | 🔴 Future |
| `requireMerkleProof` | bool | false | Require Merkle inclusion proof | 🔴 Future |
| `checkRevocation` | bool | false | Check certificate revocation via OCSP/CRL | 🔴 Future |

**How it works:**

1. Registry provides certificate fingerprint in resolution response
2. Resolver connects to agent endpoint via TLS
3. Calculates SHA-256 fingerprint of presented certificate
4. Compares with registry's fingerprint (prevents MITM attacks)

**Current limitation:** Registries must include fingerprints in resolution responses for this to work. Some registries provide certificates via separate API endpoints, but trust verification requires fingerprints embedded in the resolution response itself.

See [Trust Verification Reference](reference/trust-verification.md) for algorithm details.

---

### `rateLimit` - Rate Limiting

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | true | Enable rate limiting | 🟢 |
| `limits` | map | - | Limit profiles | 🟢 |

**Example:**
```yaml
rateLimit:
  enabled: true
  limits:
    anonymous:
      requests: 100
      window: 1m
    authenticated:
      requests: 1000
      window: 1m
```

---

### `telemetry` - Telemetry Configuration

**Type:** Object

| Field | Type | Description | Status |
|-------|------|-------------|--------|
| `logging` | LoggingConfig | Logging settings | 🟢 |
| `metrics` | MetricsConfig | Prometheus metrics | 🟢 |
| `tracing` | TracingConfig | Distributed tracing | 🔴 Future |

**Example:**
```yaml
telemetry:
  logging:
    level: info
    format: json
    output: stdout
  metrics:
    enabled: true
    port: 9091
  tracing:
    enabled: false
```

---

### `telemetry.logging` - Logging Configuration

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `level` | string | info | Log level: `debug`, `info`, `warn`, `error` | 🟢 |
| `format` | string | json | Format: `json`, `text` | 🟢 |
| `output` | string | stdout | Output: `stdout`, `stderr`, `file` | 🟢 |

---

### `telemetry.metrics` - Metrics Configuration

**Type:** Object

| Field | Type | Default | Description | Status |
|-------|------|---------|-------------|--------|
| `enabled` | bool | true | Enable Prometheus metrics | 🟢 |
| `namespace` | string | ans | Metric namespace | 🟢 |
| `subsystem` | string | resolver | Metric subsystem | 🟢 |
| `path` | string | /metrics | Metrics endpoint path | 🟢 |
| `port` | int | 9091 | Metrics server port | 🟢 |
| `host` | string | 0.0.0.0 | Metrics server host | 🟢 |

---

## Environment Variables

All string values support environment variable substitution:

**Syntax:** `${VAR_NAME:default_value}`

**Examples:**
```yaml
# Required variable (no default)
apiKey: "${GODADDY_API_KEY}"

# With default value
port: "${HTTP_PORT:8080}"
host: "${HTTP_HOST:0.0.0.0}"

# Redis connection
address: "${REDIS_ADDRESS:localhost:6379}"
password: "${REDIS_PASSWORD:}"
```

**Commonly used variables:**

- `GODADDY_API_KEY` - GoDaddy API key
- `GODADDY_API_SECRET` - GoDaddy API secret
- `REDIS_ADDRESS` - Redis server address
- `REDIS_PASSWORD` - Redis password
- `HTTP_PORT` - HTTP server port
- `LOG_LEVEL` - Logging level

---

## Complete Example Configurations

See example configurations in `configs/`:

| File | Description | Status |
|------|-------------|--------|
| `resolver-minimal-godaddy.yaml` | Minimal config with only essentials | 🟢 Recommended |
| `resolver-godaddy.yaml` | Standard GoDaddy config with in-memory cache | 🟢 Production-ready |
| `resolver-godaddy-redis.yaml` | GoDaddy with Redis cache (queue disabled) | 🟢 Production-ready |
| `resolver-queue-test.yaml` | Test config with queue enabled | 🟢 Development/Testing |
| `resolver-minimal.yaml` | Mock registry for testing | 🟢 Development |

---

## Validation

**Validate your config:**
```bash
# Check if config loads without errors
./bin/ans-resolver -config configs/your-config.yaml -validate

# Dry-run (load config but don't start server)
./bin/ans-resolver -config configs/your-config.yaml -dry-run
```

**IDE Validation:**

VS Code users can get autocomplete and validation by adding to `.vscode/settings.json`:
```json
{
  "yaml.schemas": {
    "./docs/config-schema.json": "configs/*.yaml"
  }
}
```

---

## Duration Format

All duration fields use Go duration format:

**Units:**

- `ns` - nanoseconds
- `us` - microseconds
- `ms` - milliseconds
- `s` - seconds
- `m` - minutes
- `h` - hours

**Examples:**

- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h30m` - 1.5 hours
- `100ms` - 100 milliseconds

---

## Need Help?

- **Configuration issues:** Check logs for validation errors
- **GoDaddy setup:** See `docs/AGENT_REGISTRATION.md`
- **Feature flags:** See `docs/FEATURE_FLAGS.md`
- **API documentation:** https://route-ans.github.io/route-ans
