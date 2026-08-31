# Quick Start

Get Route ANS up and running in 5 minutes.

## Prerequisites

- Go 1.23 or later (for building from source)
- Docker (optional, for containerized deployment)
- GoDaddy API credentials (for production) or use mock registry (for testing)

## Installation

### Option 1: Using Pre-built Binary

```bash
# Download the latest release
curl -LO https://github.com/route-ans/route-ans/releases/latest/download/ans-resolver-linux-amd64

# Make it executable
chmod +x ans-resolver-linux-amd64
sudo mv ans-resolver-linux-amd64 /usr/local/bin/ans-resolver

# Verify installation
ans-resolver --version
```

### Option 2: Using Docker

```bash
# Pull the image
docker pull routeans/resolver:latest

# Run with default configuration
docker run -p 8080:8080 routeans/resolver:latest
```

### Option 3: Build from Source

```bash
# Clone the repository
git clone https://github.com/route-ans/route-ans.git
cd route-ans

# Build
make build

# Binary will be in bin/ans-resolver
./bin/ans-resolver --version
```

## Running Your First Resolution

### 1. Start the Resolver

Using the minimal mock configuration (no external dependencies):

```bash
ans-resolver --config configs/resolver-minimal.yaml
```

You should see:

```
{"level":"info","time":"2026-01-17T10:00:00Z","message":"Starting ANS Resolver"}
{"level":"info","time":"2026-01-17T10:00:00Z","message":"Server listening on :8080"}
```

### 2. Resolve an ANSName

In another terminal:

```bash
# Full format (using mock registry)
curl "http://localhost:8080/v1/resolve?name=mcp://greeting.greet.PID-1234.v1.0.0.example.com"

# Simplified format (GoDaddy registry)
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com"
```

**Expected Response:**

```json
{
  "status": "active",
  "agent": "ans://v1.0.0.greeting.example.com",
  "protocol": "A2A",
  "endpoint": "https://greeting.example.com",
  "expiresAt": "2026-04-24T11:43:26+05:30"
}
```

### 3. Try Version Negotiation

Request compatible versions using semantic version ranges:

```bash
# Request any 1.x compatible version
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=^1.0.0"

# Request latest version
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=*"

# Request version >= 1.0.0
curl "http://localhost:8080/v1/resolve?name=ans://v1.0.0.greeting.example.com&version=>=1.0.0"
```

### 4. Check Health

```bash
curl http://localhost:8080/health
```

**Response:** `{"status":"healthy"}`

## What's Next?

- **[Installation Guide](installation.md)** - Detailed installation options
- **[Basic Concepts](concepts.md)** - Understand ANS terminology
- **[Tutorial](tutorial.md)** - Step-by-step walkthrough
- **[Configuration](index.md)** - Configure for your environment

## Common First Steps

### Using GoDaddy Registry

1. Get API credentials from [GoDaddy Developer Portal](https://developer.godaddy.com/keys)

2. Set environment variables:
   ```bash
   export GODADDY_API_KEY="your_key"
   export GODADDY_API_SECRET="your_secret"
   ```

3. Start with GoDaddy config:
   ```bash
   ans-resolver --config configs/resolver-godaddy.yaml
   ```

### Enable Redis Caching

1. Start Redis:
   ```bash
   docker run -d -p 6379:6379 redis:alpine
   ```

2. Start resolver with Redis:
   ```bash
   ans-resolver --config configs/resolver-godaddy-redis.yaml
   ```

### Access Swagger UI

Open your browser:
```
http://localhost:8080/swagger/
```

## Troubleshooting

**Port already in use?**
```bash
# Change port in config or use environment variable
PORT=8081 ans-resolver --config configs/resolver-minimal.yaml
```

**Can't connect to registry?**
```bash
# Check registry configuration
curl -v https://api.godaddy.com/v1/ans/health

# Use mock registry for testing
# configs/resolver-minimal.yaml uses mock by default
```

**Need help?**

- Check [Troubleshooting Guide](troubleshooting.md)
- Open an issue on [GitHub](https://github.com/route-ans/route-ans/issues)
