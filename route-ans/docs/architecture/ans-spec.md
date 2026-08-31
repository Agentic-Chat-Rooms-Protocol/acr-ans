# ANS Specification

Agent Name System (ANS) specification implementation details.

## ANS Overview

ANS provides a decentralized, secure naming system for AI agents, similar to DNS for traditional services.

## ANSName Format

### Full Format

Hierarchical naming format similar to URLs:

- Protocol prefix (mcp://, a2a://, acp://, https://)
- Capability path with optional sub-categories
- Persistent identifier (PID) for stable references
- Semantic version for compatibility management
- Fully qualified domain name for registry location

**Design Principle**: Self-describing names that encode protocol, capability, and version information.

### Simplified Format (Registry-Specific)

Some registries (like GoDaddy) use a simplified format:

```
protocol://version.host.domain
```

Example: `ans://v1.0.0.greeting.example.com`

- **Protocol**: Communication protocol (ans, a2a, etc.)
- **Version**: Semantic version (v1.0.0)
- **Host**: Fully qualified domain name

This format is automatically detected and parsed. Default values are assigned:
- Capability: `default`
- Provider ID: `PID-0000`

### Components

1. **Protocol**: Communication protocol (mcp, a2a, acp, https)
    - Determines connection semantics and message format
    - Extensible for future protocols

2. **Capability**: Agent capability (chatbot, translation, etc.)
    - Hierarchical with dot-separated sub-capabilities
    - Enables capability-based discovery

3. **Sub-capability**: Optional sub-category
    - Refinement of primary capability
    - Supports nested classification

4. **PID**: Persistent Identifier (PID-{number})
    - Stable identifier across versions
    - Enables tracking agent lifecycle

5. **Version**: Semantic version (v1.2.3)
    - **Required** in all ANSName formats
    - Enables compatibility negotiation
    - Follows semantic versioning spec
    - Used for FQDN extraction and parsing

6. **FQDN**: Fully Qualified Domain Name
    - Registry location for agent records
    - Leverages DNS infrastructure
    - Extracted from ANSName components

## Supported Protocols

### MCP (Model Context Protocol)

- **Purpose**: AI model interaction with context management
- **Usage**: Chat completions, embeddings, context-aware AI services
- **Characteristics**: Stateful conversations, context windows, model parameters

### A2A (Agent-to-Agent)

- **Purpose**: Direct agent-to-agent communication
- **Usage**: Multi-agent systems, agent collaboration, workflow orchestration
- **Characteristics**: Asynchronous messaging, agent discovery, capability negotiation

### ACP (Agent Communication Protocol)

- **Purpose**: Standardized agent messaging
- **Usage**: Interoperable agent systems, cross-platform communication
- **Characteristics**: Message schemas, protocol versioning, content negotiation

### HTTPS

- **Purpose**: Standard HTTP/REST APIs
- **Usage**: Web services, RESTful agents, HTTP-based integrations
- **Characteristics**: Request-response model, standard HTTP methods, stateless

**Extensibility**: New protocols can be added by implementing protocol handlers without changing core resolution logic.

## Resolution Process

Per ANS specification, resolution maps ANSName to endpoint:

- **Input**:
    - ANSName (must include valid version component)
    - Optional Version Range (query parameter overrides ANSName version)
- **Output**: Verified endpoint with metadata

**Note**: The version in the ANSName is required for parsing and FQDN extraction. When a version query parameter is provided, it overrides the ANSName version for negotiation.

### Steps

1. **Parse ANSName**: Extract protocol, capability, PID, version, FQDN
    - Validates format compliance
    - Decomposes into queryable components

2. **Query Registry**: Lookup agent records by FQDN
    - Retrieves all versions for the agent
    - Returns structured agent metadata

3. **Filter by PID**: Match persistent identifier
    - Ensures correct agent across versions
    - Prevents PID collisions

4. **Negotiate Version**: Select version matching range
    - Applies semantic versioning rules
    - Selects optimal compatible version

5. **Verify Trust**: Validate certificate fingerprint
    - Establishes endpoint authenticity
    - Optional based on security policy

6. **Return Result**: Endpoint URL and metadata
    - Includes protocol extensions
    - Provides expiration for caching

### Version Negotiation

Supports semantic versioning with flexible range specifiers:

**Exact Version**: Request specific version only (e.g., 1.2.3)

- Use case: Pinned dependencies, tested integrations

**Caret Range**: Compatible versions (^1.2.3 matches >=1.2.3 <2.0.0)

- Use case: Accept bug fixes and features, prevent breaking changes
- Most common for production deployments

**Tilde Range**: Patch updates (~1.2.3 matches >=1.2.3 <1.3.0)

- Use case: Accept bug fixes only
- Conservative upgrade strategy

**Comparison Operators**: Flexible constraints (>=1.0.0, <2.0.0)

- Use case: Complex version requirements
- Multiple operators can be combined

**Wildcard**: Partial version matching (1.x, 1.2.x)

- Use case: Latest within major/minor version

**Any Version**: Latest available (*)

- Use case: Development, always-latest scenarios

**Design Trade-off**: Flexible versioning enables compatibility but requires careful management to avoid breaking changes.

## Registry Storage

### TXT Record Format

Agent records stored as JSON in DNS TXT records at `_ans.{fqdn}`:

**Record Contents**:

- Full ANSName with protocol and version
- HTTPS endpoint URL for agent connection
- SHA-256 certificate fingerprint for verification
- Expiration timestamp for cache TTL management
- Protocol-specific extensions as key-value pairs

**Design Choice**: JSON encoding in TXT records balances human-readability with machine-parseability. DNS provides global distribution and caching.

### GoDaddy API

Records accessible via GoDaddy Domains REST API:

- GET endpoint for TXT record retrieval
- Authentication via API key/secret
- JSON response with record array
- Rate limiting per API tier

**Trade-off**: API provides programmatic access and real-time updates but requires service-specific integration.

## Resolution Result

Structured response containing resolved agent information:

**Fields**:

- **Status**: Verification result (verified, unverified, expired, revoked)
- **Agent**: Resolved ANSName with selected version
- **Protocol**: Communication protocol identifier
- **Endpoint**: HTTPS URL for agent connection
- **Certificate Fingerprint**: SHA-256 hash for client verification
- **Expiration**: Timestamp for cache invalidation
- **Protocol Extensions**: Custom metadata map

**Design Pattern**: **Data Transfer Object (DTO)** - structured, protocol-independent result format.

### Status Values

- **verified**: Certificate verified successfully
- **unverified**: Not verified (trust disabled)
- **expired**: Certificate expired
- **revoked**: Certificate revoked

## Trust Model

### Certificate Verification

1. **Fingerprint**: SHA-256 hash of certificate
2. **Chain Validation**: Optional CA chain verification
3. **Revocation**: CRL/OCSP checking (optional)
4. **Trust Store**: Custom trusted CAs

### Security Properties

- **Authenticity**: Prove agent identity via certificate
- **Integrity**: Fingerprint ensures certificate hasn't changed
- **Confidentiality**: TLS for endpoint communication
- **Non-repudiation**: Signed records (future)

## Caching

### Cache Key Strategy

Hierarchical keys encode all resolution parameters:

- Namespace prefix (ans:resolve:)
- Protocol identifier
- Capability and sub-capability path
- Persistent identifier (PID)
- Version or version range
- Fully qualified domain name

**Design Benefit**: Precise cache invalidation and expiration based on specific parameters.

### TTL Management

**TTL Calculation**:

- Derived from agent record expiration timestamp
- Bounded by minimum (60s) and maximum (24h) limits
- Default fallback (1h) for records without expiration

**Design Trade-off**: Short TTL ensures freshness but increases registry load. Longer TTL improves performance but may serve stale data.

**Cache Invalidation**: Time-based expiration (no manual invalidation required).

## Error Codes

| Code | Error | Description |
|------|-------|-------------|
| 400 | Bad Request | Invalid ANSName format |
| 404 | Not Found | Agent not found in registry |
| 422 | Unprocessable | Verification failed |
| 500 | Internal Error | Server error |
| 503 | Service Unavailable | Registry unavailable |

## Protocol Extensions

Custom per-protocol metadata extends base resolution result:

**Purpose**: Protocol-specific parameters and capabilities

- Model configuration (temperature, max tokens)
- Rate limits and quotas
- Supported features and versions
- Service-level agreements

**MCP Extensions**: Model parameters, token limits, temperature settings

**A2A Extensions**: Supported languages, payload size limits, protocol versions

**Design Pattern**: **Extension Object Pattern** - allows protocol-specific data without polluting core schema.

**Extensibility**: New protocols define their own extension schema without modifying resolver.

## Batch Resolution

Resolve multiple agents in single request for efficiency:

- **Request**: Array of ANSNames to resolve
- **Response**: Map of ANSName to resolution result or error

**Benefits**:

- Reduced HTTP overhead (single round-trip)
- Parallel processing on server
- Atomic operation for dependent resolutions

**Behavior**:

- Individual resolution errors don't fail entire batch
- Partial results returned on timeout
- Results returned in any order (not sequential)

**Design Trade-off**: Batch operations improve throughput but complicate error handling compared to sequential requests.

## ANS Spec Compliance

Route ANS implements:

- ✅ ANS Spec Section 2: ANSName Format
- ✅ ANS Spec Section 3: Protocol Support
- ✅ ANS Spec Section 4: Resolution Algorithm
- ✅ ANS Spec Section 5: Version Negotiation
- ✅ ANS Spec Section 6: Trust Verification
- ✅ ANS Spec Section 7: Caching

## Comparison to DNS

| Feature | DNS | ANS |
|---------|-----|-----|
| **Purpose** | Map hostnames to IPs | Map agent names to endpoints |
| **Records** | A, AAAA, CNAME, etc. | TXT with JSON |
| **Versioning** | None | Semantic versioning |
| **Security** | DNSSEC (optional) | Certificate fingerprints |
| **Protocols** | TCP/UDP | HTTP/HTTPS, A2A, MCP |
| **Caching** | TTL-based | TTL + expiration |

## Reference

- ANS Specification: [Link to spec](https://www.ietf.org/archive/id/draft-narajala-ans-00.html)
- Semantic Versioning: https://semver.org/
- TLS Fingerprinting: RFC 6698

## Next Steps

- **[Components](components.md)** - Implementation details
- **[Resolution Flow](resolution-flow.md)** - Request lifecycle
- **[Version Negotiation](../reference/version-negotiation.md)** - Version negotiation guide
