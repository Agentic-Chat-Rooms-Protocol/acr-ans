# Registry Adapters Reference

Complete reference for registry adapter implementations.

## Overview

Registry adapters connect Route ANS to backend agent registries. Currently supported:

- **GoDaddy**: GoDaddy Domains API
- **Mock**: Testing/development

## Interface Definition

```go
type Registry interface {
    // Lookup a single agent by exact ANSName
    Lookup(ctx context.Context, ansName string) (*models.AgentRecord, error)
    
    // Lookup all agents for a given FQDN (for version negotiation)
    LookupByFQDN(ctx context.Context, fqdn string) ([]*models.AgentRecord, error)
    
    // Register a new agent (optional)
    Register(ctx context.Context, record *models.AgentRecord) error
    
    // Deregister an agent (optional)
    Deregister(ctx context.Context, ansName string) error
}
```

## GoDaddy Registry

### Configuration

```yaml
registry:
  type: godaddy
  godaddy:
    api_key: ${GODADDY_API_KEY}
    secret: ${GODADDY_SECRET}
    base_url: https://api.godaddy.com
    timeout: 30s
    retry_attempts: 3
    retry_delay: 1s
```

### Environment Variables

```bash
export GODADDY_API_KEY=your_api_key
export GODADDY_SECRET=your_secret
```

### API Endpoints

**Resolution:**
```
POST /v1/agents/resolution
```

Request body:
```json
{
  "agentHost": "greeting.example.com",
  "version": "1.0.0"
}
```

Response (200):
```json
{
  "ansName": "ans://v1.0.0.greeting.example.com",
  "links": [
    {
      "rel": "agent-details",
      "href": "https://api.godaddy.com/v1/agents/{agentId}"
    }
  ]
}
```

**Version Negotiation:**

GoDaddy supports semantic version ranges:

```json
{
  "agentHost": "greeting.example.com",
  "version": "^1.0.0"
}
```

Supported ranges: `*`, `^1.0.0`, `~1.2.3`, `>=1.0.0`, exact versions

GoDaddy's API automatically selects the best matching version.

**Agent Details:**
```
GET /v1/agents/{agentId}
```

### Record Format

Response includes agent metadata, endpoint, and certificate information retrieved through linked resources.

### Error Handling

| Status | Error | Action |
|--------|-------|--------|
| 401 | Authentication failed | Check API credentials |
| 403 | Authorization failed | Verify API permissions |
| 404 | Agent not found | Agent not registered |
| 422 | Invalid request | Check agentHost and version format |
| 429 | Rate limit | Retry with backoff |
| 500 | Server error | Retry operation |

### Rate Limits

- **Production**: 60 requests/minute
- **OTE (Test)**: 600 requests/minute

### Best Practices

1. **Caching**: Always enable caching to reduce API calls
2. **Batch**: Use batch lookups when possible
3. **Retries**: Implement exponential backoff
4. **Monitoring**: Track API usage metrics

## Mock Registry

### Purpose

For testing and development without external dependencies.

### Configuration

```yaml
registry:
  type: mock
  mock:
    records:
      - ans_name: "mcp://test.PID-123.v1.0.0.example.com"
        endpoint: "https://test.example.com:8443"
        cert_fingerprint: "SHA256:test123"
        expires_at: "2025-12-31T23:59:59Z"
```

### Programmatic Usage

```go
mockRegistry := registry.NewMockRegistry()

// Add test record
mockRegistry.AddRecord(&models.AgentRecord{
    ANSName:         "mcp://test.PID-123.v1.0.0.example.com",
    Endpoint:        "https://test.example.com:8443",
    CertFingerprint: "SHA256:test123",
})

// Use in resolver
resolver := resolver.New(mockRegistry, cache, verifier)
```

## DNS Registry (Future)

Direct DNS TXT record lookup:

```yaml
registry:
  type: dns
  dns:
    resolvers:
      - 8.8.8.8
      - 1.1.1.1
    timeout: 5s
```

Query:

```bash
dig TXT _ans.example.com
```

## Custom Registry Implementation

### Example: Database Registry

```go
type DatabaseRegistry struct {
    db *sql.DB
}

func (d *DatabaseRegistry) Lookup(ctx context.Context, ansName string) (*models.AgentRecord, error) {
    query := `
        SELECT ans_name, endpoint, cert_fingerprint, expires_at, extensions
        FROM agent_records
        WHERE ans_name = $1 AND expires_at > NOW()
    `
    
    var record models.AgentRecord
    err := d.db.QueryRowContext(ctx, query, ansName).Scan(
        &record.ANSName,
        &record.Endpoint,
        &record.CertFingerprint,
        &record.ExpiresAt,
        &record.Extensions,
    )
    
    return &record, err
}

func (d *DatabaseRegistry) LookupByFQDN(ctx context.Context, fqdn string) ([]*models.AgentRecord, error) {
    query := `
        SELECT ans_name, endpoint, cert_fingerprint, expires_at, extensions
        FROM agent_records
        WHERE fqdn = $1 AND expires_at > NOW()
        ORDER BY version DESC
    `
    
    rows, err := d.db.QueryContext(ctx, query, fqdn)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var records []*models.AgentRecord
    for rows.Next() {
        var r models.AgentRecord
        rows.Scan(&r.ANSName, &r.Endpoint, &r.CertFingerprint, &r.ExpiresAt, &r.Extensions)
        records = append(records, &r)
    }
    
    return records, nil
}
```

### Database Schema

```sql
CREATE TABLE agent_records (
    id SERIAL PRIMARY KEY,
    ans_name VARCHAR(512) UNIQUE NOT NULL,
    protocol VARCHAR(16) NOT NULL,
    capability VARCHAR(128) NOT NULL,
    pid VARCHAR(32) NOT NULL,
    version VARCHAR(32) NOT NULL,
    fqdn VARCHAR(256) NOT NULL,
    endpoint VARCHAR(512) NOT NULL,
    cert_fingerprint VARCHAR(128) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    extensions JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_fqdn (fqdn),
    INDEX idx_expires (expires_at)
);
```

## Registry Metrics

All registries expose:

```conf
# Lookup duration
ans_registry_lookup_duration_seconds{registry="godaddy|mock"}

# Error count
ans_registry_errors_total{registry="godaddy",error_type="timeout|not_found"}

# Request count
ans_registry_requests_total{registry="godaddy",operation="lookup|register"}
```

## Next Steps

- **[Cache Providers](cache-providers.md)** - Cache implementations
- **[Trust Verification](trust-verification.md)** - Certificate verification
- **[Extending](../development/extending.md)** - Custom implementations
