<div align="center">
  <img src="docs/assets/logo.png" alt="ANS Logo" width="200"/>
  
  # ANS Resolution Server

  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
  [![Go Report Card](https://goreportcard.com/badge/github.com/route-ans/route-ans)](https://goreportcard.com/report/github.com/route-ans/route-ans)
  [![Go Version](https://img.shields.io/github/go-mod/go-version/route-ans/route-ans)](go.mod)
  [![Release](https://img.shields.io/github/v/release/route-ans/route-ans)](https://github.com/route-ans/route-ans/releases)
  [![Container Image](https://img.shields.io/badge/ghcr.io-route--ans-blue?logo=docker)](https://github.com/route-ans/route-ans/pkgs/container/route-ans)
  [![Build Status](https://img.shields.io/github/actions/workflow/status/route-ans/route-ans/ci.yml?branch=main)](https://github.com/route-ans/route-ans/actions)
  [![Documentation](https://img.shields.io/badge/docs-mkdocs-blue)](https://route-ans.github.io/route-ans/)
  
</div>

A high-performance, cryptographically-verified resolution service for the Agent Name Service (ANS) ecosystem. The resolution server translates ANSName identifiers into verified agent endpoints, acting as the "DNS resolver" layer for autonomous AI agents.

## Overview

The ANS Resolution Server provides:

- **Fast Resolution**: Sub-50ms cache-hit lookups, <100ms P99 latency
- **Cryptographic Verification**: Mandatory signature and Merkle proof validation
- **Protocol Agnostic**: Supports A2A, MCP, ACP, HTTPS, and custom protocols
- **Pluggable Architecture**: Swap cache, queue, store, and registry providers
- **Cloud Native**: Declarative YAML configuration, Kubernetes-ready
- **High Availability**: Stateless design for horizontal scaling

## Architecture

```
┌──────────────┐
│ Client Agent │
└──────┬───────┘
       │ ANS Query: GET /v1/resolve?name=mcp://...
       ▼
┌────────────────────────────┐
│   ANS Resolution Server    │
│────────────────────────────│
│ 1. API Gateway (Chi)       │
│ 2. Cache Layer (Pluggable) │
│ 3. ANSName Parser          │
│ 4. Registry Selector       │
│ 5. Registry Adapters       │
│ 6. Trust Verifier          │
│ 7. Policy Engine           │
└────────┬───────────┬───────┘
         │           │
         ▼           ▼
 ┌─────────────┐  ┌──────────────┐
 │ Registry A  │  │ Registry B   │
 │ (GoDaddy)   │  │ (Veritrust)  │
 └─────────────┘  └──────────────┘
```

## Quick Start

### Prerequisites

- Go 1.25+
- GoDaddy API credentials (for production) or use mock registry (for testing)
- Optional: Redis for distributed caching

### Running Locally

```bash
# Clone the repository
git clone https://github.com/route-ans/route-ans.git
cd route-ans

# Install dependencies
go mod download

# Run with default config (mock registry)
make run

# Or build and run
make build
./bin/ans-resolver --config configs/resolver-minimal.yaml
```

### Using GoDaddy Registry

1. **Get API Credentials**: Sign up at [GoDaddy Developer Portal](https://developer.godaddy.com/keys)

2. **Set Environment Variables**:
```bash
export GODADDY_API_KEY="your_api_key"
export GODADDY_API_SECRET="your_api_secret"
```

3. **Start the Resolver**:
```bash
./bin/ans-resolver --config configs/resolver-godaddy.yaml
```

4. **Test Resolution**:
```bash
# Resolve using simplified format (GoDaddy registry)
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com"

# Version negotiation with semantic versioning
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=^1.0.0"
```

### Registering Your Agent

Generate certificates and register with GoDaddy:

```bash
# Generate CSRs for your agent
openssl req -new -newkey rsa:2048 -nodes \
  -keyout agent-server.key -out agent-server.csr \
  -subj "/CN=myagent.example.com/O=MyCompany/C=US"

openssl req -new -newkey rsa:2048 -nodes \
  -keyout agent-identity.key -out agent-identity.csr \
  -subj "/CN=a2a:\/\/myagent.capability.PID-XXX.v1.0.0.example.com/O=MyCompany/C=US"

# Register with GoDaddy
curl -X POST "https://api.godaddy.com/v1/agents/register" \
  -H "Authorization: sso-key YOUR_API_KEY:YOUR_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "protocol": "a2a",
    "agentName": "myagent",
    "agentCapability": "capability",
    "provider": "PID-XXX",
    "version": "1.0.0",
    "extension": "example.com",
    "serverCsrPEM": "BASE64_ENCODED_CSR",
    "identityCsrPEM": "BASE64_ENCODED_CSR",
    "agentCategory": "AI/ML"
  }'
```

See [AGENT_REGISTRATION.md](AGENT_REGISTRATION.md) for detailed registration instructions.

### Using Docker

```bash
# Build the image
make docker-build

# Run the container
docker run -p 8080:8080 -p 9091:9091 ans-resolution-server:latest
```

### Using Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -k deployments/kubernetes/
```

## API Reference

### Resolve an ANSName

Resolve an agent's ANSName to its verified endpoint:

```bash
# Full format (mock registry)
GET /v1/resolve?name=a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev

# Simplified format (GoDaddy registry)
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com
```

**Response:**
```json
{
  "status": "active",
  "agent": "ans://v1.0.0.greeting.example.com",
  "protocol": "A2A",
  "endpoint": "https://greeting.example.com",
  "expiresAt": "2026-04-24T11:43:26+05:30"
}
```

### ANSName Formats

**Full Format** (7+ components):
```
protocol://agentName.capability.providerID.version.extension
```
Example: `mcp://chatbot.conversation.PID-5678.v1.2.3.example.com`

**Simplified Format** (GoDaddy and compatible registries):
```
protocol://version.host.domain
```
Example: `ans://v1.0.0.greeting.example.com`

The parser automatically detects which format is used.

**Important**: A valid version component is **required** in all ANSName formats, even when using version negotiation. The version in the ANSName is used for parsing and FQDN extraction. Use the `version` query parameter to override version selection.

### Version Negotiation

The resolver supports semantic version ranges for flexible version resolution.

#### Supported Version Range Formats

| Format | Example | Description |
|--------|---------|-------------|
| **Exact** | `1.0.0` | Exact version match |
| **Greater than** | `>1.0.0` | Any version higher than 1.0.0 |
| **Greater or equal** | `>=1.0.0` | Version 1.0.0 or higher |
| **Less than** | `<2.0.0` | Any version below 2.0.0 |
| **Less or equal** | `<=2.0.0` | Version 2.0.0 or lower |
| **Caret** | `^1.2.3` | Compatible changes (>=1.2.3 <2.0.0) |
| **Tilde** | `~1.2.3` | Patch-level changes (>=1.2.3 <1.3.0) |
| **Major wildcard** | `1.x` | Any minor/patch in version 1 |
| **Minor wildcard** | `1.2.x` | Any patch in version 1.2 |
| **Any version** | `*` | Latest available version |

#### Usage Examples

**Request latest version:**
```bash
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=*
```

**Request compatible version (caret):**
```bash
# Allows 1.2.3, 1.2.4, 1.5.0 but not 2.0.0
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=^1.2.0
```

**Request patch updates only (tilde):**
```bash
# Allows 1.2.3, 1.2.4 but not 1.3.0
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=~1.2.3
```

**Request with comparison:**
```bash
# Greater than or equal to 1.0.0
GET /v1/resolve?name=ans://v1.0.0.greeting.example.com&version=>=1.0.0
```

**Registry Support:**
- **GoDaddy**: Native semantic versioning support - version negotiation happens server-side
- **Mock Registry**: Local version negotiation with full semver support

### Batch Resolution

**Request version range:**
```bash
# Get highest version between 1.0.0 and 2.0.0
GET /v1/resolve?name=a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev&version=>=1.0.0&version=<2.0.0
```

#### Version Negotiation Strategy

When multiple versions match the range:
1. **Filter** - Only versions matching the range are considered
2. **Sort** - Candidates are sorted by semantic version
3. **Select** - The **highest matching version** is returned
4. **Cache** - Result is cached with standard TTL

**Example:**
- Available: v1.0.0, v1.1.0, v1.2.0, v2.0.0
- Range: `^1.0.0`
- Result: `v1.2.0` (highest compatible with 1.x)

### Swagger UI

Interactive API documentation available at:
```
http://localhost:8080/swagger/
```

Or generate updated docs:
```bash
make docs
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Verified - agent found and trusted |
| 400 | Bad Request - invalid ANSName format |
| 404 | Not Found - agent not registered |
| 421 | Untrusted Agent - signature verification failed |
| 422 | Certificate Revoked |
| 429 | Rate Limited |
| 500 | Internal Server Error |

### Other Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /ready` | Readiness check |
| `GET /v1/resolve` | Resolve ANSName to endpoint |
| `POST /v1/resolve/batch` | Batch resolution |
| `GET /v1/stats` | Resolver statistics |

## Configuration

The server uses a declarative YAML configuration with environment variable substitution:

```yaml
apiVersion: ans.io/v1alpha1
kind: ResolverConfig
metadata:
  name: ans-resolver

spec:
  server:
    http:
      port: ${HTTP_PORT:8080}
      readTimeout: 30s

  # Pluggable cache provider
  cache:
    provider: ${CACHE_PROVIDER:memory}  # memory | redis | memcached
    ttl:
      default: 5m
      verified: 10m
    redis:
      address: ${REDIS_ADDRESS:localhost:6379}

  # Pluggable queue provider (disabled by default)
  queue:
    enabled: ${QUEUE_ENABLED:false}  # Enable for event processing
    provider: ${QUEUE_PROVIDER:memory}  # memory | redis-streams

  # Pluggable store provider
  store:
    provider: ${STORE_PROVIDER:memory}  # memory | postgres | sqlite

  # Registry adapters
  registries:
    - name: primary
      type: ans-registry
      config:
        baseUrl: https://registry.ans.io

  # Trust verification
  trust:
    provider: file  # file | vault | k8s-secret
    verification:
      requireSignature: true
      requireMerkleProof: true
      checkRevocation: true
```

See `configs/resolver.yaml` for the complete configuration reference.

## Pluggable Providers

### Cache Providers

| Provider | Description | Config Key |
|----------|-------------|------------|
| `memory` | In-memory LRU cache | Default |
| `redis` | Redis cache | `cache.redis.*` |
| `memcached` | Memcached | `cache.memcached.*` |

### Queue Providers

**Note:** Queue is disabled by default. Enable it to process registry events for cache invalidation and distributed coordination.

| Provider | Description | Config Key | Status |
|----------|-------------|------------|--------|
| `memory` | In-memory channel | Default | ✅ Implemented |
| `redis-streams` | Redis Streams | `queue.redis-streams.*` | ⚙️ Config only |

### Store Providers

| Provider | Description | Config Key |
|----------|-------------|------------|
| `memory` | In-memory map | Default |
| `postgres` | PostgreSQL | `store.postgres.*` |
| `sqlite` | SQLite | `store.sqlite.*` |

### Registry Adapters

| Adapter | Description | Status |
|---------|-------------|--------|
| `godaddy` | GoDaddy ANS Registry | ✅ Implemented |
| `mock` | Mock adapter for testing | ✅ Implemented |
| `veritrust` | DID/Verifiable Credentials | 🔜 Planned |

**Using GoDaddy Registry:**
```yaml
registries:
  - name: godaddy
    type: godaddy
    enabled: true
    config:
      baseURL: "https://api.godaddy.com/v1"
      apiKey: "${GODADDY_API_KEY}"
      apiSecret: "${GODADDY_API_SECRET}"
```

### Trust Providers

| Provider | Description |
|----------|-------------|
| `file` | File-based trust store |
| `vault` | HashiCorp Vault |
| `k8s-secret` | Kubernetes Secret |

## Event Processing

The resolver includes an optional queue system for processing registry events:

**What it does:**
- Listens for registry events (registered, renewed, revoked, deprecated)
- Automatically invalidates cache when agents are updated
- Tracks event processing metrics
- Supports distributed coordination via Redis Streams

**Enable queue processing:**
```yaml
queue:
  enabled: true
  provider: memory  # or redis-streams for distributed setups
  bufferSize: 100
```

**Supported event types:**
- `registered` - New agent registered
- `renewed` - Agent certificate renewed
- `revoked` - Agent revoked
- `deprecated` - Agent marked deprecated
- `expired` - Agent expired

When disabled (default), the resolver operates in pure request-response mode with cache-based optimization.

## Observability

### Metrics (Prometheus)

```bash
# Available at /metrics or port 9091
curl http://localhost:9091/metrics
```

Key metrics:
- `ans_resolver_resolution_total` - Resolution requests by status
- `ans_resolver_resolution_duration_seconds` - Resolution latency histogram
- `ans_resolver_cache_hits_total` - Cache hit count
- `ans_resolver_registry_lookups_total` - Registry lookup count

### Tracing (OpenTelemetry)

Enable tracing in config:

```yaml
telemetry:
  tracing:
    enabled: true
    provider: otlp
    sampleRate: 0.1
    otlp:
      endpoint: localhost:4317
```

### Logging

```yaml
telemetry:
  logging:
    level: info  # debug | info | warn | error
    format: json  # json | text | pretty
```

## Development

### Prerequisites

- Go 1.22+
- Docker (optional)
- Redis (for redis providers)

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Run linter
make lint
```

### Project Structure

```
ans-resolution-server/
├── cmd/resolver/          # Entry point
├── internal/
│   ├── cache/             # Cache provider interface & implementations
│   ├── config/            # Configuration loading
│   ├── queue/             # Queue provider interface & implementations
│   ├── registry/          # Registry adapter interface & implementations
│   ├── resolver/          # Core resolution logic
│   ├── server/            # HTTP server
│   ├── store/             # Store provider interface & implementations
│   ├── telemetry/         # Logging, metrics, tracing
│   └── trust/             # Trust verification
├── pkg/ansname/           # Public ANSName parser
├── configs/               # Configuration files
├── deployments/
│   ├── docker/            # Dockerfile
│   └── kubernetes/        # K8s manifests
└── api/                   # API specifications
```

## License

Apache 2.0 - See [LICENSE](../LICENSE)

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md)
