# Extending Route ANS

Guide for extending Route ANS with custom components.

## Overview

Route ANS is designed for extensibility through interfaces. You can implement custom:

- Registry adapters
- Cache providers
- Trust verifiers
- Queue backends

## Creating a Custom Registry

### 1. Implement the Interface

```go
// internal/registry/custom.go
package registry

import (
    "context"
    "github.com/route-ans/route-ans/internal/models"
)

type CustomRegistry struct {
    // Your configuration
    apiEndpoint string
    apiKey      string
}

func NewCustomRegistry(endpoint, apiKey string) *CustomRegistry {
    return &CustomRegistry{
        apiEndpoint: endpoint,
        apiKey:      apiKey,
    }
}

// Lookup implements Registry.Lookup
func (c *CustomRegistry) Lookup(ctx context.Context, ansName string) (*models.AgentRecord, error) {
    // Parse ANSName
    name, err := ansname.Parse(ansName)
    if err != nil {
        return nil, err
    }
    
    // Query your backend
    record, err := c.queryBackend(ctx, name.FQDN())
    if err != nil {
        return nil, err
    }
    
    return record, nil
}

// LookupByFQDN implements Registry.LookupByFQDN
func (c *CustomRegistry) LookupByFQDN(ctx context.Context, fqdn string) ([]*models.AgentRecord, error) {
    // Query all records for FQDN
    records, err := c.queryAllVersions(ctx, fqdn)
    return records, err
}

// Register implements Registry.Register (optional)
func (c *CustomRegistry) Register(ctx context.Context, record *models.AgentRecord) error {
    return fmt.Errorf("not implemented")
}

// Deregister implements Registry.Deregister (optional)
func (c *CustomRegistry) Deregister(ctx context.Context, ansName string) error {
    return fmt.Errorf("not implemented")
}
```

### 2. Add Configuration

```go
// internal/config/config.go
type RegistryConfig struct {
    Type   string `yaml:"type"`
    GoDaddy GoDaddyConfig `yaml:"godaddy,omitempty"`
    Custom  CustomConfig  `yaml:"custom,omitempty"`  // Add this
}

type CustomConfig struct {
    Endpoint string `yaml:"endpoint"`
    APIKey   string `yaml:"api_key"`
}
```

### 3. Register in Factory

```go
// internal/resolver/resolver.go
func newRegistry(cfg *config.Config) (registry.Registry, error) {
    switch cfg.Registry.Type {
    case "godaddy":
        return registry.NewGoDaddyRegistry(cfg.Registry.GoDaddy)
    case "custom":
        return registry.NewCustomRegistry(
            cfg.Registry.Custom.Endpoint,
            cfg.Registry.Custom.APIKey,
        )
    case "mock":
        return registry.NewMockRegistry()
    default:
        return nil, fmt.Errorf("unknown registry type: %s", cfg.Registry.Type)
    }
}
```

### 4. Configuration Example

```yaml
registry:
  type: custom
  custom:
    endpoint: https://api.myregistry.com
    api_key: ${CUSTOM_REGISTRY_KEY}
```

## Creating a Custom Cache

### 1. Implement Interface

```go
// internal/cache/custom.go
package cache

import (
    "context"
    "time"
    "github.com/route-ans/route-ans/internal/resolver"
)

type CustomCache struct {
    client YourCacheClient
}

func NewCustomCache(config CustomCacheConfig) *CustomCache {
    return &CustomCache{
        client: NewYourClient(config),
    }
}

func (c *CustomCache) Get(ctx context.Context, key string) (*resolver.ResolutionResult, error) {
    data, err := c.client.Get(ctx, key)
    if err != nil {
        return nil, err
    }
    
    var result resolver.ResolutionResult
    json.Unmarshal(data, &result)
    return &result, nil
}

func (c *CustomCache) Set(ctx context.Context, key string, value *resolver.ResolutionResult, ttl time.Duration) error {
    data, _ := json.Marshal(value)
    return c.client.Set(ctx, key, data, ttl)
}

func (c *CustomCache) Delete(ctx context.Context, key string) error {
    return c.client.Delete(ctx, key)
}

func (c *CustomCache) Clear(ctx context.Context) error {
    return c.client.FlushAll(ctx)
}

func (c *CustomCache) Size() int64 {
    return c.client.Size()
}
```

### 2. Add to Factory

```go
func newCache(cfg *config.Config) (cache.Cache, error) {
    switch cfg.Cache.Type {
    case "memory":
        return cache.NewMemoryCache(cfg.Cache.Memory)
    case "redis":
        return cache.NewRedisCache(cfg.Cache.Redis)
    case "custom":
        return cache.NewCustomCache(cfg.Cache.Custom)
    default:
        return nil, fmt.Errorf("unknown cache type: %s", cfg.Cache.Type)
    }
}
```

## Creating a Custom Trust Verifier

```go
// internal/trust/custom.go
package trust

import (
    "context"
    "crypto/x509"
)

type CustomVerifier struct {
    // Your config
}

func (v *CustomVerifier) Verify(ctx context.Context, endpoint, expectedFingerprint string) error {
    // Custom verification logic
    // - Connect to endpoint
    // - Retrieve certificate
    // - Validate against your criteria
    return nil
}

func (v *CustomVerifier) ValidateCertificate(cert *x509.Certificate) error {
    // Custom certificate validation
    return nil
}

func (v *CustomVerifier) LoadTrustStore(path string) error {
    // Load custom trust store
    return nil
}
```

## Adding Custom Metrics

```go
// internal/telemetry/metrics.go
import "github.com/prometheus/client_golang/prometheus"

var (
    customMetric = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ans_custom_operations_total",
            Help: "Custom operation counter",
        },
        []string{"operation", "status"},
    )
)

func init() {
    prometheus.MustRegister(customMetric)
}

// Use in your code
customMetric.WithLabelValues("lookup", "success").Inc()
```

## Adding Custom Middleware

```go
// internal/server/middleware.go
func CustomMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-processing
        start := time.Now()
        
        // Call next handler
        next.ServeHTTP(w, r)
        
        // Post-processing
        duration := time.Since(start)
        log.Info("request completed", "duration", duration)
    })
}

// Add to server
func NewServer(resolver resolver.Resolver, config *config.Config) *Server {
    r := chi.NewRouter()
    r.Use(CustomMiddleware)
    // ... other setup
}
```

## Creating Protocol Extensions

For MCP protocol with custom extensions:

```go
// internal/resolver/mcp_extensions.go
type MCPExtensions struct {
    Model       string  `json:"model"`
    MaxTokens   int     `json:"max_tokens"`
    Temperature float64 `json:"temperature"`
    // Add custom fields
    CustomField string  `json:"custom_field"`
}

func parseMCPExtensions(ext map[string]interface{}) *MCPExtensions {
    // Parse extensions from agent record
    // Add to ResolutionResult.ProtocolExtensions
}
```

## Plugin System (Future)

Planned support for dynamically loaded plugins:

```go
// pkg/plugin/interface.go
type Plugin interface {
    Name() string
    Version() string
    Init(config map[string]interface{}) error
    
    // Hooks
    OnResolve(ctx context.Context, ansName string) error
    OnResult(ctx context.Context, result *resolver.ResolutionResult) error
}
```

## Testing Custom Components

```go
// internal/registry/custom_test.go
func TestCustomRegistry_Lookup(t *testing.T) {
    // Setup
    registry := NewCustomRegistry("http://test", "key")
    
    // Test
    result, err := registry.Lookup(context.Background(), "mcp://test.PID-123.v1.0.0.example.com")
    
    // Assert
    require.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "https://agent.example.com:8443", result.Endpoint)
}
```

## Contributing Components

To contribute your component:

1. Fork repository
2. Implement interface
3. Add tests (80%+ coverage)
4. Update documentation
5. Submit pull request

### PR Checklist

- [ ] Interface fully implemented
- [ ] Unit tests added
- [ ] Integration tests added
- [ ] Documentation updated
- [ ] Example configuration provided
- [ ] Error handling implemented
- [ ] Metrics added

## Best Practices

1. **Error Handling**: Return descriptive errors
2. **Context**: Respect context cancellation
3. **Metrics**: Expose relevant metrics
4. **Logging**: Use structured logging
5. **Configuration**: Validate config on startup
6. **Testing**: Include unit and integration tests
7. **Documentation**: Document public APIs

## Example: Complete Custom Registry

See `examples/custom-registry/` for a complete working example.

```bash
cd examples/custom-registry
go run main.go
```

## Next Steps

- **[Testing](testing.md)** - Testing guide
- **[Components](../architecture/components.md)** - Component architecture
