# Component Details

Detailed documentation for each Route ANS Resolver component.

## HTTP Server (`internal/server/`)

### Purpose
Handles HTTP API requests and provides REST endpoints.

### Files

- `http.go`: HTTP server, routes, handlers

### Responsibilities

**Request Processing**:

- Extract and validate query parameters
- Parse ANSName and version ranges
- Handle content negotiation (JSON/YAML)
- Return structured responses with appropriate status codes

**Middleware Chain**:

- Request logging with correlation IDs
- Rate limiting per client IP/API key
- Authentication and authorization
- CORS policy enforcement
- Metrics collection for observability

**Design Pattern**: **Middleware Chain Pattern** allows composable request processing stages that can be reordered or disabled via configuration.

**Error Handling**: Consistent error responses with problem details (RFC 7807) for client debugging.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/resolve` | Resolve single ANSName |
| POST | `/v1/resolve/batch` | Resolve multiple ANSNames |
| GET | `/v1/agent/{ansName}` | Get agent information |
| GET | `/v1/agent/{ansName}/verify` | Verify agent certificate |
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/metrics` | Prometheus metrics |

## Resolver Core (`internal/resolver/`)

### Purpose
Core resolution logic and version negotiation.

### Files

- `resolver.go`: Main resolver implementation
- `types.go`: Data types and interfaces

### Responsibilities

**Resolution Orchestration**:

- Coordinate cache, registry, and trust verifier interactions
- Implement resolution algorithm per ANS specification
- Handle single and batch resolution workflows
- Manage resolution lifecycle and error recovery

**Version Negotiation**:

- Parse semantic version ranges (caret, tilde, operators)
- Filter candidate versions from registry results
- Select optimal version based on semantic versioning rules
- Support exact matches and wildcard ranges

**Design Pattern**: **Facade Pattern** - provides simple interface to complex subsystems (cache, registry, verifier).

**Design Trade-off**: Eager verification adds latency but ensures security. Optional lazy verification mode trades security for performance.

**Caching Strategy**:

- Cache keys include version range for precise invalidation
- TTL derived from agent record expiration
- Cache-aside pattern: check cache, fetch on miss, populate cache

### Metrics

- `ans_resolver_requests_total{status="success|failure"}`
- `ans_resolver_request_duration_seconds{operation="resolve|batch"}`
- `ans_resolver_version_negotiations_total`

## Cache Layer (`internal/cache/`)

### Purpose
Cache resolution results to improve performance.

### Files

- `interface.go`: Cache interface
- `memory.go`: In-memory implementation
- `redis.go`: Redis implementation

### Responsibilities

**Cache Management**:

- Store and retrieve resolution results
- Automatic TTL-based expiration
- Size-based eviction (LRU for memory cache)
- Namespace isolation for multi-tenant deployments

**Design Pattern**: **Strategy Pattern** - swap cache implementations (memory/Redis) without changing resolver code.

### Memory Cache

**Implementation**: In-memory LRU cache with concurrent access support

- Thread-safe operations using sync.Map
- Configurable max size and entry limits
- Background goroutine for TTL expiration cleanup
- Best for single-instance deployments

**Design Trade-off**: Fast but not shared across instances. Suitable for development and small deployments.

### Redis Cache

**Implementation**: Distributed cache using Redis

- JSON serialization for structured data
- Native Redis TTL for automatic expiration
- Connection pooling for performance
- Best for multi-instance, high-availability deployments

**Design Trade-off**: Network latency overhead but shared state enables horizontal scaling.

### Cache Key Strategy

Hierarchical keys enable selective invalidation:

- Include protocol, capability, PID, version, FQDN
- Namespace prefix prevents key collisions
- Version range included for precise cache hits

### Metrics

- `ans_cache_hits_total`
- `ans_cache_misses_total`
- `ans_cache_size_bytes`
- `ans_cache_entries_total`

## Registry Adapter (`internal/registry/`)

### Purpose
Interface with external agent registries (GoDaddy, DNS, etc.).

### Files

- `interface.go`: Registry interface
- `godaddy.go`: GoDaddy implementation
- `mock.go`: Mock for testing

### Responsibilities

**Registry Integration**:

- Lookup agent records from external sources
- Parse and validate registry responses
- Handle registry-specific authentication
- Support multiple registry backends

**Design Pattern**: **Adapter Pattern** - converts registry-specific APIs into unified interface.

**Extensibility Points**:

- Implement custom registry adapters (Ethereum ENS, IPFS, DNS TXT, private databases)
- Support multi-registry fallback chains
- Add registry health monitoring and circuit breakers

### GoDaddy Implementation

**Integration Approach**:
- Uses GoDaddy Domains REST API
- Fetches TXT records from `_ans` subdomain
- Parses JSON-encoded agent records
- Implements API key authentication
- Connection pooling for performance

**Design Choice**: API-based lookup provides centralized control and real-time updates compared to DNS propagation delays.

### Record Format

Agent records are stored as JSON in TXT records with standardized schema:

- ANS name with full protocol and version
- HTTPS endpoint URL
- SHA-256 certificate fingerprint
- Expiration timestamp for cache TTL
- Protocol-specific extensions (metadata)

### Metrics

- `ans_registry_lookup_duration_seconds{registry="godaddy"}`
- `ans_registry_errors_total{registry="godaddy",error_type="timeout"}`
- `ans_registry_requests_total{registry="godaddy"}`

## Trust Verifier (`internal/trust/`)

### Purpose
Verify agent certificates and fingerprints.

### Files

- `interface.go`: Verifier interface
- `verifier.go`: Implementation

### Responsibilities

**Certificate Verification**:

- Connect to agent endpoint and retrieve TLS certificate
- Calculate SHA-256 fingerprint of certificate
- Compare against expected fingerprint from registry
- Optionally validate certificate chain and revocation status

**Design Pattern**: **Template Method Pattern** - defines verification skeleton, allowing subclasses to customize validation steps.

### Verification Process

**Multi-stage Verification**:

1. **TLS Handshake**: Establish secure connection to agent endpoint
2. **Certificate Extraction**: Retrieve peer certificate from connection
3. **Fingerprint Calculation**: Compute SHA-256 hash of certificate DER bytes
4. **Comparison**: Verify calculated fingerprint matches registry value
5. **Optional Chain Validation**: Check certificate against trust store

**Design Trade-off**: Manual fingerprint verification bypasses traditional CA trust model, enabling self-signed certificates while maintaining security through registry-based trust.

### Verification Modes

- **Strict**: Full certificate validation including chain, expiration, and revocation
- **Permissive**: Fingerprint-only verification, skip CA validation
- **Disabled**: No verification (testing/development only)

**Extensibility**: Add custom verification policies (HSM integration, hardware token verification, blockchain-based trust).

### Metrics

- `ans_verifier_operations_total{result="verified|unverified|error"}`
- `ans_verifier_duration_seconds`

## Queue System (`internal/queue/`)

### Purpose
Asynchronous task processing (optional).

### Files

- `interface.go`: Queue interface
- `memory.go`: In-memory queue
- `redis.go`: Redis Streams queue

### Responsibilities

**Task Management**:

- Enqueue tasks for asynchronous processing
- Dequeue tasks with consumer group coordination
- Acknowledge successful processing
- Negative acknowledge for retry

**Design Pattern**: **Producer-Consumer Pattern** - decouples task generation from execution.

### Use Cases

- **Background Verification**: Verify certificates asynchronously to reduce request latency
- **Batch Processing**: Process large resolution batches in background
- **Cache Warming**: Preload frequently accessed agents
- **Metrics Aggregation**: Collect and process telemetry data

**Design Trade-off**: Queue adds complexity but enables scaling and reliability through asynchronous processing.

## Telemetry (`internal/telemetry/`)

### Purpose
Observability: logging, metrics, tracing.

### Files

- `logging.go`: Structured logging
- `metrics.go`: Prometheus metrics

### Responsibilities

**Structured Logging**:

- JSON-formatted logs with contextual fields
- Correlation IDs for request tracing
- Log levels (debug, info, warn, error)
- Sensitive data redaction

**Metrics Collection**:

- Prometheus-compatible metrics export
- Request counters by status and operation
- Response time histograms
- Active request gauges
- Cache hit/miss ratios

**Design Pattern**: **Observer Pattern** - components emit events, telemetry system records them without tight coupling.

**Observability Trade-off**: Detailed metrics provide insights but add overhead. Configurable sampling rates balance detail with performance.

## Models (`internal/models/`)

### Purpose
Shared data types and structures.

### Files

- `types.go`: Core data types

### Responsibilities

**Data Models**:

- ANSName structure with parsed components
- AgentRecord with metadata and extensions
- VersionRange for semantic version matching
- ResolutionResult with status and endpoint info

**Design Pattern**: **Value Object Pattern** - immutable data structures that represent domain concepts.

**Design Choice**: Centralized models prevent duplication and ensure consistency across components.

## Configuration (`internal/config/`)

### Purpose
Configuration management and validation.

### Files

- `config.go`: Config struct and loader

### Responsibilities

**Configuration Loading**:

- Load from YAML files
- Override with environment variables
- Validate required fields and constraints
- Provide sensible defaults

**Design Pattern**: **Builder Pattern** - incrementally construct configuration from multiple sources with validation.

**Hierarchy**: File → Environment → Flags (highest priority)

**Design Trade-off**: Multiple configuration sources increase flexibility but add complexity. Clear precedence rules prevent confusion.

## Next Steps

- **[Resolution Flow](resolution-flow.md)** - Request lifecycle
- **[ANS Spec](ans-spec.md)** - Specification details
- **[Extending](../development/extending.md)** - Add custom components
