# Troubleshooting Guide

Common issues and solutions for Route ANS Resolver.

## Resolution Errors

### Agent Not Found (404)

**Symptom:**
```json
{
  "error": "agent not found",
  "ans_name": "mcp://unknown.PID-123.v1.0.0.example.com"
}
```

**Causes:**
- ANSName doesn't exist in registry
- Registry misconfigured
- DNS record not published

**Solutions:**
```bash
# 1. Verify ANSName format
echo "mcp://capability.PID-123.v1.0.0.example.com"

# 2. Check registry configuration
cat configs/resolver.yaml | grep -A 5 registry

# 3. Test registry directly (GoDaddy)
curl -X GET "https://api.godaddy.com/v1/domains/example.com/records/TXT/_ans" \
  -H "Authorization: sso-key $GODADDY_API_KEY:$GODADDY_SECRET"

# 4. Check DNS resolution
dig TXT _ans.example.com
```

### Version Not Found

**Symptom:**
```json
{
  "error": "no matching version found",
  "requested_range": "^2.0.0",
  "available_versions": ["1.0.0", "1.1.0", "1.2.0"]
}
```

**Solutions:**
```bash
# 1. Use wildcard for latest
curl "http://localhost:8080/v1/resolve?name=mcp://agent.PID-123.v1.0.0.example.com&version=*"

# 2. Try specific version
curl "http://localhost:8080/v1/resolve?name=mcp://agent.PID-123.v1.0.0.example.com&version=1.0.0"

# 3. Check version format
# Valid: 1.0.0, ^1.0.0, ~1.2.0, >=1.0.0, *
# Invalid: v1.0.0, 1.0, latest
```

### Verification Failed (422)

**Symptom:**
```json
{
  "error": "certificate verification failed",
  "reason": "fingerprint mismatch"
}
```

**Solutions:**
```bash
# 1. Check trust verification settings
grep -A 5 "trust:" configs/resolver.yaml

# 2. Disable verification for testing (NOT production)
# In config:
trust:
  enabled: false

# 3. Update trust store
cp new-cert.pem /etc/resolver/certs/trusted/

# 4. Verify certificate fingerprint
openssl x509 -in cert.pem -noout -fingerprint -sha256
```

## Connection Errors

### Cannot Connect to Resolver

**Symptom:**
```bash
curl: (7) Failed to connect to localhost port 8080
```

**Solutions:**
```bash
# 1. Check if resolver is running
ps aux | grep ans-resolver
systemctl status ans-resolver

# 2. Check listening ports
netstat -tlnp | grep 8080
ss -tlnp | grep 8080

# 3. Check firewall
ufw status
iptables -L -n

# 4. Check logs
journalctl -u ans-resolver -f
docker logs ans-resolver
```

### Timeout Errors

**Symptom:**
```bash
curl: (28) Operation timed out after 30000 milliseconds
```

**Solutions:**
```bash
# 1. Increase timeout
curl --max-time 60 http://localhost:8080/v1/resolve?name=...

# 2. Check registry response time
time curl -X GET https://api.godaddy.com/...

# 3. Check network connectivity
ping api.godaddy.com
traceroute api.godaddy.com

# 4. Adjust config timeout
# In config:
registry:
  godaddy:
    timeout: 30s  # Increase if needed
```

### Redis Connection Failed

**Symptom:**
```
ERROR: failed to connect to Redis: dial tcp :6379: connect: connection refused
```

**Solutions:**
```bash
# 1. Check Redis is running
docker ps | grep redis
redis-cli ping

# 2. Verify connection settings
grep -A 5 "cache:" configs/resolver.yaml

# 3. Test Redis connection
redis-cli -h localhost -p 6379 ping

# 4. Check network (Kubernetes)
kubectl get svc redis -n ans-system
kubectl exec -it ans-resolver-pod -- nc -zv redis 6379

# 5. Fallback to memory cache
# In config:
cache:
  type: memory  # Instead of redis
```

## Performance Issues

### High Latency

**Symptom:**
Response times > 1 second

**Diagnosis:**
```bash
# Check metrics
curl http://localhost:9090/metrics | grep duration

# Test with timing
time curl http://localhost:8080/v1/resolve?name=...

# Check cache hit rate
curl http://localhost:9090/metrics | grep cache_hits
curl http://localhost:9090/metrics | grep cache_misses
```

**Solutions:**
```yaml
# 1. Enable caching
cache:
  type: redis
  ttl: 3600

# 2. Increase cache size
cache:
  memory:
    max_size_mb: 512

# 3. Add cache warming
# Pre-populate frequently accessed agents

# 4. Tune registry timeout
registry:
  godaddy:
    timeout: 10s
    retry_attempts: 2
```

### High Memory Usage

**Symptom:**
```bash
docker stats  # Shows high memory
# ans-resolver  512MiB / 512MiB  100%
```

**Solutions:**
```bash
# 1. Check cache size
curl http://localhost:9090/metrics | grep cache_size

# 2. Reduce cache size
# In config:
cache:
  memory:
    max_size_mb: 256
    max_entries: 10000

# 3. Check for memory leaks
# Enable pprof
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof -http=:8081 heap.prof

# 4. Increase container limits
# In Docker:
docker run -m 1g ...

# In Kubernetes:
resources:
  limits:
    memory: 1Gi
```

### High CPU Usage

**Solutions:**
```bash
# 1. Check request rate
curl http://localhost:9090/metrics | grep requests_total

# 2. Enable rate limiting
server:
  rate_limit:
    enabled: true
    requests_per_second: 100

# 3. Profile CPU
curl http://localhost:8080/debug/pprof/profile > cpu.prof
go tool pprof cpu.prof

# 4. Scale horizontally
kubectl scale deployment ans-resolver --replicas=3
```

## Configuration Issues

### Config File Not Found

**Symptom:**
```
ERROR: failed to load config: open /etc/resolver/config.yaml: no such file
```

**Solutions:**
```bash
# 1. Specify config path
ans-resolver --config /path/to/config.yaml

# 2. Check file exists
ls -l /etc/resolver/config.yaml

# 3. Check permissions
chmod 644 /etc/resolver/config.yaml

# 4. Use environment variables instead
export ANS_SERVER_PORT=8080
export ANS_CACHE_TYPE=memory
```

### Invalid Configuration

**Symptom:**
```
ERROR: invalid configuration: registry type "unknown" not supported
```

**Solutions:**
```bash
# 1. Validate YAML syntax
yamllint configs/resolver.yaml

# 2. Check against example
diff configs/resolver.yaml configs/resolver-minimal.yaml

# 3. Review supported values
# registry.type: godaddy, mock
# cache.type: memory, redis
# queue.type: memory, redis
```

## Docker Issues

### Container Exits Immediately

**Symptom:**
```bash
docker ps -a  # Shows Exited (1)
```

**Solutions:**
```bash
# 1. Check logs
docker logs ans-resolver

# 2. Run interactively
docker run -it --entrypoint sh ghcr.io/route-ans/route-ans:latest

# 3. Check config mount
docker run -v $(pwd)/configs:/configs ... --config /configs/resolver.yaml

# 4. Verify environment variables
docker run --env-file .env ...
```

### Image Pull Errors

**Solutions:**
```bash
# 1. Authenticate
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# 2. Use specific tag
docker pull ghcr.io/route-ans/route-ans:1.0.0

# 3. Check image exists
curl https://ghcr.io/v2/route-ans/resolver/tags/list
```

## Kubernetes Issues

### Pods Not Starting

**Symptom:**
```bash
kubectl get pods -n ans-system
# NAME               READY   STATUS             RESTARTS
# ans-resolver-xxx   0/1     CrashLoopBackOff   5
```

**Solutions:**
```bash
# 1. Check pod logs
kubectl logs -n ans-system ans-resolver-xxx

# 2. Describe pod
kubectl describe pod -n ans-system ans-resolver-xxx

# 3. Check events
kubectl get events -n ans-system --sort-by='.lastTimestamp'

# 4. Verify ConfigMap
kubectl get configmap -n ans-system resolver-config -o yaml

# 5. Check resource limits
kubectl top pods -n ans-system
```

### Service Not Accessible

**Solutions:**
```bash
# 1. Check service
kubectl get svc -n ans-system
kubectl describe svc ans-resolver -n ans-system

# 2. Test from another pod
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://ans-resolver.ans-system.svc.cluster.local:8080/health

# 3. Check endpoints
kubectl get endpoints -n ans-system ans-resolver

# 4. Verify ingress
kubectl get ingress -n ans-system
kubectl describe ingress ans-resolver -n ans-system
```

## Debugging Tools

### Enable Debug Logging

```bash
# Runtime
curl -X POST http://localhost:8080/debug/log-level?level=debug

# Config
logging:
  level: debug
```

### Health Check Script

```bash
#!/bin/bash
# health-check.sh

echo "=== ANS Resolver Health Check ==="

# 1. Process check
if pgrep ans-resolver > /dev/null; then
    echo "✓ Process running"
else
    echo "✗ Process not running"
    exit 1
fi

# 2. HTTP health
if curl -sf http://localhost:8080/health > /dev/null; then
    echo "✓ Health endpoint OK"
else
    echo "✗ Health endpoint failed"
    exit 1
fi

# 3. Resolution test
if curl -sf "http://localhost:8080/v1/resolve?name=mcp://test.PID-123.v1.0.0.example.com" > /dev/null; then
    echo "✓ Resolution working"
else
    echo "⚠ Resolution failed (may be expected)"
fi

# 4. Metrics
if curl -sf http://localhost:9090/metrics | grep -q ans_resolver_requests_total; then
    echo "✓ Metrics exposed"
else
    echo "✗ Metrics not available"
fi

echo "=== Check complete ==="
```

### Common Commands

```bash
# View all logs
journalctl -u ans-resolver -f --no-pager

# Search logs for errors
journalctl -u ans-resolver | grep ERROR

# Export metrics
curl -s http://localhost:9090/metrics > metrics.txt

# Test resolution with verbose output
curl -v http://localhost:8080/v1/resolve?name=...

# Trace requests
curl -H "X-Trace-ID: debug-123" http://localhost:8080/v1/resolve?name=...
# Then search logs for trace-id=debug-123
```

## Getting Help

### Information to Collect

When reporting issues, include:

1. **Version**: `ans-resolver --version`
2. **Configuration**: `cat configs/resolver.yaml`
3. **Logs**: Recent error logs
4. **Environment**: OS, Docker version, Kubernetes version
5. **Request**: Full curl command that fails
6. **Response**: Complete error response

### Support Channels

- GitHub Issues: https://github.com/route-ans/route-ans/issues
- Documentation: https://route-ans.github.io/route-ans/
- Slack: [Join community](#)

## Next Steps

- **[Monitoring](monitoring.md)** - Set up monitoring
- **[Security](security.md)** - Security best practices
- **[Deployment](deployment.md)** - Production deployment
