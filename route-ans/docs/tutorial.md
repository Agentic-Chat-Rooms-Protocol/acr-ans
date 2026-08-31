# Tutorial

Step-by-step tutorial for using Route ANS Resolver.

## Overview

In this tutorial, you'll:

1. Set up a local resolver
2. Resolve your first ANSName
3. Experiment with version negotiation
4. Configure caching
5. Monitor resolver metrics

**Time Required:** 20 minutes

## Prerequisites

- Route ANS Resolver installed ([Installation Guide](installation.md))
- `curl` or similar HTTP client
- Text editor

## Part 1: First Resolution

### Step 1: Start the Resolver

Start with the minimal configuration (uses mock registry):

```bash
ans-resolver --config configs/resolver-minimal.yaml
```

Expected output:
```json
{"level":"info","message":"Starting ANS Resolver","version":"1.0.0"}
{"level":"info","message":"Cache provider: memory"}
{"level":"info","message":"Registry: mock"}
{"level":"info","message":"Server listening","address":":8080"}
```

### Step 2: Check Health

```bash
curl http://localhost:8080/health
```

Response:
```json
{"status":"healthy"}
```

### Step 3: Your First Resolution

```bash
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.0.0.example.com"
```

**Response:**
```json
{
  "status": "verified",
  "agent": "mcp://greeting.greet.PID-1234.v1.0.0.example.com",
  "protocol": "mcp",
  "endpoint": "https://greeting.example.com",
  "certFingerprint": "sha256:abc123...",
  "expiresAt": "2026-03-15T10:30:00Z",
  "protocolExtensions": {
    "mcp": {
      "url": "https://greeting.example.com/mcp",
      "capabilities": ["conversation", "completion"]
    }
  },
  "cached": false
}
```

### Step 4: Verify Caching

Run the same request again:

```bash
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.0.0.example.com"
```

Note: Response is now instant (cached).

## Part 2: Version Negotiation

### Step 1: Request Latest Version

```bash
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.0.0.example.com&version=*"
```

The resolver returns the highest available version.

### Step 2: Request Compatible Version

```bash
# Get any 1.x version (caret range)
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.0.0.example.com&version=^1.0.0"
```

### Step 3: Request Patch Updates

```bash
# Get patch updates only (tilde range)
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.2.0.example.com&version=~1.2.0"
```

**Explanation:**

- `*` - Latest available version
- `^1.0.0` - Compatible with 1.0.0 (allows 1.x.x)
- `~1.2.0` - Patch updates only (allows 1.2.x)
- `1.x` - Any minor/patch in version 1

See [Version Negotiation](reference/version-negotiation.md) for details.

## Part 3: Batch Resolution

### Step 1: Create Request File

Create `batch-request.json`:

```json
{
  "names": [
    "mcp://greeting.greet.PID-1234.v1.0.0.example.com",
    "a2a://translate.translation.PID-5678.v2.0.0.example.com",
    "acp://analyze.analysis.PID-9012.v1.5.0.example.com"
  ]
}
```

### Step 2: Send Batch Request

```bash
curl -X POST http://localhost:8080/v1/resolve/batch \
  -H "Content-Type: application/json" \
  -d @batch-request.json
```

**Response:**
```json
{
  "results": {
    "mcp://greeting.greet.PID-1234.v1.0.0.example.com": {
      "status": "verified",
      "endpoint": "https://greeting.example.com"
    },
    "a2a://translate.translation.PID-5678.v2.0.0.example.com": {
      "status": "verified",
      "endpoint": "https://translate.example.com"
    },
    "acp://analyze.analysis.PID-9012.v1.5.0.example.com": {
      "status": "verified",
      "endpoint": "https://analyze.example.com"
    }
  }
}
```

## Part 4: Using Real Registry (GoDaddy)

### Step 1: Get API Credentials

1. Visit [GoDaddy Developer Portal](https://developer.godaddy.com/keys)
2. Create an API key
3. Note your API Key and Secret

### Step 2: Configure Environment

```bash
export GODADDY_API_KEY="your_api_key_here"
export GODADDY_API_SECRET="your_api_secret_here"
```

### Step 3: Stop Mock Resolver

```bash
# Stop the resolver (Ctrl+C)
```

### Step 4: Start with GoDaddy Registry

```bash
ans-resolver --config configs/resolver-godaddy.yaml
```

### Step 5: Resolve Real Agent

```bash
# Replace with actual registered agent
curl "http://localhost:8080/v1/resolve?name=a2a://your-agent.capability.PID-XXX.v1.0.0.yourdomain.com"
```

## Part 5: Redis Caching

### Step 1: Start Redis

```bash
docker run -d --name redis -p 6379:6379 redis:alpine
```

### Step 2: Update Configuration

Edit `configs/resolver-godaddy-redis.yaml`:

```yaml
cache:
  provider: redis
  redis:
    address: localhost:6379
    password: ""
    db: 0
```

### Step 3: Restart Resolver

```bash
ans-resolver --config configs/resolver-godaddy-redis.yaml
```

### Step 4: Verify Redis Usage

```bash
# Make a resolution request
curl "http://localhost:8080/v1/resolve?name=mcp://agent.PID-123.v1.0.0.example.com"

# Check Redis
docker exec redis redis-cli KEYS "*"
```

You should see cache keys in Redis.

## Part 6: Monitoring

### Step 1: Check Statistics

```bash
curl http://localhost:8080/v1/stats
```

**Response:**
```json
{
  "totalRequests": 42,
  "cacheHits": 28,
  "cacheMisses": 14,
  "cacheHitRate": 0.67,
  "registryLookups": 14,
  "verificationSuccess": 13,
  "verificationFailures": 1,
  "averageLatencyMs": 45.3
}
```

### Step 2: Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

Response includes Prometheus-format metrics:
```
# HELP ans_resolver_requests_total Total number of resolution requests
# TYPE ans_resolver_requests_total counter
ans_resolver_requests_total{status="success"} 40
ans_resolver_requests_total{status="error"} 2

# HELP ans_resolver_cache_hits_total Cache hits
# TYPE ans_resolver_cache_hits_total counter
ans_resolver_cache_hits_total 28
```

## Part 7: Interactive API Exploration

### Step 1: Open Swagger UI

Navigate to:
```
http://localhost:8080/swagger/
```

### Step 2: Try API Endpoints

1. Click on `/v1/resolve` endpoint
2. Click "Try it out"
3. Enter an ANSName
4. Click "Execute"
5. View response

## Common Tasks

### Task: Invalidate Cache

```bash
# Invalidate specific agent
curl -X DELETE "http://localhost:8080/v1/cache?name=mcp://agent.PID-123.v1.0.0.example.com"
```

## Troubleshooting

### Issue: Connection Refused

**Problem:** Can't connect to resolver

**Solution:**
```bash
# Check if resolver is running
curl http://localhost:8080/health

# Check logs
# If using systemd:
sudo journalctl -u ans-resolver -f
```

### Issue: Registry Not Found

**Problem:** `404: agent not found`

**Solution:**

1. Verify agent is registered in the registry
2. Check registry configuration
3. Use mock registry for testing

### Issue: Verification Failed

**Problem:** `422: verification failed`

**Solution:**

1. Check certificate validity
2. Verify trust anchor configuration
3. Temporarily use permissive verification mode (development only)

## Next Steps

Congratulations! You've completed the tutorial. Next:

- **[Configuration Reference](index.md)** - Deep dive into configuration
- **[Deployment](deployment.md)** - Deploy to production
- **[Monitoring](monitoring.md)** - Set up comprehensive monitoring
- **[Security](security.md)** - Secure your deployment

## Code Examples

### Python Client

```python
import requests

def resolve_agent(ans_name, version_range=None):
    params = {'name': ans_name}
    if version_range:
        params['version'] = version_range
    
    response = requests.get(
        'http://localhost:8080/v1/resolve',
        params=params
    )
    return response.json()

# Usage
result = resolve_agent(
    'mcp://agent.PID-123.v1.0.0.example.com',
    version_range='^1.0.0'
)
print(f"Endpoint: {result['endpoint']}")
```

### Node.js Client

```javascript
const axios = require('axios');

async function resolveAgent(ansName, versionRange) {
  const params = { name: ansName };
  if (versionRange) params.version = versionRange;
  
  const response = await axios.get(
    'http://localhost:8080/v1/resolve',
    { params }
  );
  return response.data;
}

// Usage
const result = await resolveAgent(
  'mcp://agent.PID-123.v1.0.0.example.com',
  '^1.0.0'
);
console.log(`Endpoint: ${result.endpoint}`);
```

### Go Client

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
)

func resolveAgent(ansName, versionRange string) (map[string]interface{}, error) {
    params := url.Values{}
    params.Add("name", ansName)
    if versionRange != "" {
        params.Add("version", versionRange)
    }
    
    resp, err := http.Get(fmt.Sprintf(
        "http://localhost:8080/v1/resolve?%s",
        params.Encode(),
    ))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}

// Usage
result, _ := resolveAgent(
    "mcp://agent.PID-123.v1.0.0.example.com",
    "^1.0.0",
)
fmt.Printf("Endpoint: %v\n", result["endpoint"])
```
