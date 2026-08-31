# Testing Guide

Comprehensive testing guide for Route ANS.

## Testing Stack

- **Framework**: Go testing package
- **Assertions**: testify/assert, testify/require
- **Mocking**: testify/mock, gomock
- **Coverage**: go test -cover

## Running Tests

### All Tests

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...

# Specific package
go test ./internal/resolver/
```

### Coverage Report

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# Coverage by package
go test -coverprofile=coverage.out ./... && \
  go tool cover -func=coverage.out
```

## Unit Tests

### Resolver Tests

```go
// internal/resolver/resolver_test.go
func TestResolver_Resolve(t *testing.T) {
    // Setup mocks
    mockRegistry := &MockRegistry{}
    mockCache := &MockCache{}
    mockVerifier := &MockVerifier{}
    
    resolver := &DefaultResolver{
        registry:      mockRegistry,
        cache:         mockCache,
        trustVerifier: mockVerifier,
    }
    
    // Mock expectations
    mockCache.On("Get", mock.Anything, mock.Anything).
        Return(nil, errors.New("not found"))
    
    mockRegistry.On("Lookup", mock.Anything, mock.Anything).
        Return(&models.AgentRecord{
            ANSName:         "mcp://test.PID-123.v1.0.0.example.com",
            Endpoint:        "https://test.com",
            CertFingerprint: "abc123",
        }, nil)
    
    mockVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).
        Return(nil)
    
    mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
        Return(nil)
    
    // Test
    result, err := resolver.Resolve(context.Background(), "mcp://test.PID-123.v1.0.0.example.com")
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "verified", result.Status)
    assert.Equal(t, "https://test.com", result.Endpoint)
    
    // Verify mocks
    mockRegistry.AssertExpectations(t)
    mockCache.AssertExpectations(t)
}
```

### Version Negotiation Tests

```go
// pkg/ansname/version_test.go
func TestNegotiateVersion(t *testing.T) {
    tests := []struct {
        name       string
        records    []*models.AgentRecord
        range      string
        expected   string
        shouldFail bool
    }{
        {
            name: "caret range",
            records: []*models.AgentRecord{
                {Version: "1.0.0"},
                {Version: "1.2.3"},
                {Version: "2.0.0"},
            },
            range:    "^1.0.0",
            expected: "1.2.3",
        },
        {
            name: "no match",
            records: []*models.AgentRecord{
                {Version: "1.0.0"},
            },
            range:      "^2.0.0",
            shouldFail: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NegotiateVersion(tt.records, tt.range)
            if tt.shouldFail {
                assert.Nil(t, result)
            } else {
                require.NotNil(t, result)
                assert.Equal(t, tt.expected, result.Version)
            }
        })
    }
}
```

## Integration Tests

### HTTP API Tests

```go
// internal/server/http_test.go
func TestHTTP_Resolve(t *testing.T) {
    // Setup server with mock resolver
    mockResolver := &MockResolver{}
    server := NewServer(mockResolver, &config.Config{})
    
    // Create test server
    ts := httptest.NewServer(server.Handler())
    defer ts.Close()
    
    // Mock response
    mockResolver.On("Resolve", mock.Anything, mock.Anything).
        Return(&resolver.ResolutionResult{
            Status:   "verified",
            Endpoint: "https://test.com",
        }, nil)
    
    // Make request
    resp, err := http.Get(ts.URL + "/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    // Assert response
    assert.Equal(t, 200, resp.StatusCode)
    
    var result resolver.ResolutionResult
    json.NewDecoder(resp.Body).Decode(&result)
    assert.Equal(t, "verified", result.Status)
}
```

### End-to-End Tests

```go
// test/e2e/resolver_test.go
func TestE2E_Resolution(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }
    
    // Start resolver
    cfg := &config.Config{
        Server: config.ServerConfig{Port: 8080},
        Cache:  config.CacheConfig{Type: "memory"},
        Registry: config.RegistryConfig{Type: "mock"},
    }
    
    resolver := resolver.New(cfg)
    server := server.NewServer(resolver, cfg)
    
    go server.Start()
    defer server.Stop()
    
    // Wait for server
    time.Sleep(100 * time.Millisecond)
    
    // Test resolution
    resp, err := http.Get("http://localhost:8080/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, 200, resp.StatusCode)
}
```

## Benchmark Tests

```go
// internal/resolver/resolver_bench_test.go
func BenchmarkResolver_Resolve(b *testing.B) {
    resolver := setupResolver()
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        resolver.Resolve(ctx, "mcp://test.PID-123.v1.0.0.example.com")
    }
}

func BenchmarkCache_Get(b *testing.B) {
    cache := cache.NewMemoryCache(cache.MemoryCacheConfig{})
    ctx := context.Background()
    
    // Populate cache
    cache.Set(ctx, "key", &resolver.ResolutionResult{}, time.Hour)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cache.Get(ctx, "key")
    }
}
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./...
```

## Table-Driven Tests

```go
func TestParseANSName(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        want      *ANSName
        wantError bool
    }{
        {
            name:  "valid MCP",
            input: "mcp://chatbot.PID-123.v1.0.0.example.com",
            want: &ANSName{
                Protocol:   "mcp",
                Capability: "chatbot",
                PID:        "PID-123",
                Version:    "1.0.0",
                FQDN:       "example.com",
            },
        },
        {
            name:      "invalid format",
            input:     "invalid",
            wantError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseANSName(tt.input)
            if tt.wantError {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

## Mock Interfaces

### Creating Mocks

```go
// internal/registry/mock.go
type MockRegistry struct {
    mock.Mock
}

func (m *MockRegistry) Lookup(ctx context.Context, ansName string) (*models.AgentRecord, error) {
    args := m.Called(ctx, ansName)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*models.AgentRecord), args.Error(1)
}

func (m *MockRegistry) LookupByFQDN(ctx context.Context, fqdn string) ([]*models.AgentRecord, error) {
    args := m.Called(ctx, fqdn)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*models.AgentRecord), args.Error(1)
}
```

## Test Fixtures

```go
// test/fixtures/fixtures.go
func NewTestAgentRecord() *models.AgentRecord {
    return &models.AgentRecord{
        ANSName:         "mcp://test.PID-123.v1.0.0.example.com",
        Protocol:        "mcp",
        Capability:      "test",
        PID:             "PID-123",
        Version:         "1.0.0",
        FQDN:            "example.com",
        Endpoint:        "https://test.example.com:8443",
        CertFingerprint: "SHA256:abc123",
        ExpiresAt:       time.Now().Add(24 * time.Hour),
    }
}
```

## Continuous Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## Test Coverage Goals

- **Overall**: >80%
- **Critical paths**: >90%
- **Core resolver**: >95%
- **Utilities**: >70%

## Testing Best Practices

1. **Arrange-Act-Assert**: Structure tests clearly
2. **Table-Driven**: Use for multiple scenarios
3. **Mocks**: Isolate dependencies
4. **Fixtures**: Reuse test data
5. **Cleanup**: Always defer cleanup
6. **Parallel**: Use `t.Parallel()` when safe
7. **Context**: Test context cancellation
8. **Errors**: Test both success and failure paths

## Manual Testing

### Local Testing

```bash
# Start resolver
make run

# Test resolution
curl "http://localhost:8080/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com"

# Test with version negotiation
curl "http://localhost:8080/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com&version=^1.0.0"

# Batch resolution
curl -X POST http://localhost:8080/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d '{"names": ["mcp://test.PID-123.v1.0.0.example.com"]}'
```

### Load Testing

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Run load test
hey -n 10000 -c 100 "http://localhost:8080/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com"
```

## Next Steps

- **[Extending](extending.md)** - Add custom components
- **[Components](../architecture/components.md)** - Architecture
