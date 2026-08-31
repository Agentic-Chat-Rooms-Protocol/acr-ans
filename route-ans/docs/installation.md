# Installation

Complete installation guide for Route ANS Resolver.

## System Requirements

### Minimum Requirements
- **CPU**: 1 core
- **Memory**: 512 MB RAM
- **Disk**: 100 MB
- **OS**: Linux, macOS, Windows

### Recommended for Production
- **CPU**: 2+ cores
- **Memory**: 2 GB RAM
- **Disk**: 1 GB (for logs)
- **OS**: Linux (Ubuntu 20.04+, CentOS 8+, Debian 11+)

### Dependencies
- **Go 1.23+** (if building from source)
- **Redis** (optional, for distributed caching)
- **PostgreSQL** (optional, for persistent storage - if enabled)

## Installation Methods

### 1. Pre-built Binaries

Download the latest release for your platform:

**Linux (amd64)**
```bash
curl -LO https://github.com/route-ans/route-ans/releases/latest/download/ans-resolver-linux-amd64
chmod +x ans-resolver-linux-amd64
sudo mv ans-resolver-linux-amd64 /usr/local/bin/ans-resolver
```

**Linux (arm64)**
```bash
curl -LO https://github.com/route-ans/route-ans/releases/latest/download/ans-resolver-linux-arm64
chmod +x ans-resolver-linux-arm64
sudo mv ans-resolver-linux-arm64 /usr/local/bin/ans-resolver
```

**macOS (Intel)**
```bash
curl -LO https://github.com/route-ans/route-ans/releases/latest/download/ans-resolver-darwin-amd64
chmod +x ans-resolver-darwin-amd64
sudo mv ans-resolver-darwin-amd64 /usr/local/bin/ans-resolver
```

**macOS (Apple Silicon)**
```bash
curl -LO https://github.com/route-ans/route-ans/releases/latest/download/ans-resolver-darwin-arm64
chmod +x ans-resolver-darwin-arm64
sudo mv ans-resolver-darwin-arm64 /usr/local/bin/ans-resolver
```

**Verify Installation**
```bash
ans-resolver --version
```

### 2. Docker

**Pull from Docker Hub**
```bash
docker pull routeans/resolver:latest
```

**Run with Default Configuration**
```bash
docker run -d \
  --name ans-resolver \
  -p 8080:8080 \
  routeans/resolver:latest
```

**Run with Custom Configuration**
```bash
docker run -d \
  --name ans-resolver \
  -p 8080:8080 \
  -v /path/to/your/config.yaml:/etc/ans/config.yaml \
  -e GODADDY_API_KEY=your_key \
  -e GODADDY_API_SECRET=your_secret \
  routeans/resolver:latest \
  --config /etc/ans/config.yaml
```

**Docker Compose**
```yaml
version: '3.8'
services:
  resolver:
    image: routeans/resolver:latest
    ports:
      - "8080:8080"
    environment:
      - GODADDY_API_KEY=${GODADDY_API_KEY}
      - GODADDY_API_SECRET=${GODADDY_API_SECRET}
    volumes:
      - ./configs/resolver.yaml:/etc/ans/config.yaml
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    restart: unless-stopped
```

### 3. Build from Source

**Clone Repository**
```bash
git clone https://github.com/route-ans/route-ans.git
cd route-ans
```

**Install Dependencies**
```bash
go mod download
```

**Build**
```bash
make build
```

Binary will be created at `bin/ans-resolver`

**Install Globally**
```bash
sudo make install
```

**Build Docker Image**
```bash
make docker-build
```

### 4. Kubernetes

**Using kubectl**
```bash
# Apply manifests
kubectl apply -f deployments/kubernetes/
```

See [Deployment Guide](deployment.md) for detailed Kubernetes setup.

## Configuration

### 1. Create Configuration File

```bash
# Copy example configuration
cp configs/resolver-minimal.yaml /etc/ans/config.yaml

# Edit configuration
vi /etc/ans/config.yaml
```

### 2. Set Environment Variables

```bash
export GODADDY_API_KEY="your_api_key"
export GODADDY_API_SECRET="your_api_secret"
export REDIS_ADDRESS="localhost:6379"
```

### 3. Run Resolver

```bash
ans-resolver --config /etc/ans/config.yaml
```

Or with systemd:

```bash
sudo systemctl start ans-resolver
sudo systemctl enable ans-resolver
```

## Systemd Service (Linux)

Create `/etc/systemd/system/ans-resolver.service`:

```ini
[Unit]
Description=ANS Resolution Server
After=network.target

[Service]
Type=simple
User=ans
Group=ans
ExecStart=/usr/local/bin/ans-resolver --config /etc/ans/config.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ans-resolver

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/ans

[Install]
WantedBy=multi-user.target
```

**Enable and start:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable ans-resolver
sudo systemctl start ans-resolver
sudo systemctl status ans-resolver
```

## Verification

### Check Version
```bash
ans-resolver --version
```

### Health Check
```bash
curl http://localhost:8080/health
```

### Test Resolution
```bash
curl "http://localhost:8080/v1/resolve?name=mcp://test.capability.PID-123.v1.0.0.example.com"
```

### View Logs
```bash
# If running with systemd
sudo journalctl -u ans-resolver -f

# If running with Docker
docker logs -f ans-resolver
```

## Upgrading

### Binary
```bash
# Download new version
curl -LO https://github.com/route-ans/route-ans/releases/download/v1.2.0/ans-resolver-linux-amd64
chmod +x ans-resolver-linux-amd64
sudo systemctl stop ans-resolver
sudo mv ans-resolver-linux-amd64 /usr/local/bin/ans-resolver
sudo systemctl start ans-resolver
```

### Docker
```bash
docker pull routeans/resolver:latest
docker stop ans-resolver
docker rm ans-resolver
docker run -d --name ans-resolver -p 8080:8080 routeans/resolver:latest
```

### Kubernetes
```bash
kubectl rollout restart deployment/ans-resolver -n ans-system
```

## Uninstallation

### Binary
```bash
sudo systemctl stop ans-resolver
sudo systemctl disable ans-resolver
sudo rm /usr/local/bin/ans-resolver
sudo rm /etc/systemd/system/ans-resolver.service
sudo systemctl daemon-reload
```

### Docker
```bash
docker stop ans-resolver
docker rm ans-resolver
docker rmi routeans/resolver:latest
```

## Next Steps

- **[Configuration Reference](configuration.md)** - Configure the resolver
- **[Tutorial](tutorial.md)** - Learn by example
- **[Deployment Guide](deployment.md)** - Production deployment
- **[Monitoring](monitoring.md)** - Set up monitoring and alerts
