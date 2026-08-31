# Monitoring and Observability

Comprehensive monitoring guide for Route ANS Resolver.

## Metrics

The resolver exposes Prometheus metrics on `/metrics` endpoint (default port 9090).

### Available Metrics

#### Request Metrics

```conf
# Total resolution requests
ans_resolver_requests_total{status="success|failure"}

# Request duration histogram
ans_resolver_request_duration_seconds{operation="resolve|batch"}

# Active requests
ans_resolver_active_requests{operation="resolve|batch"}
```

#### Cache Metrics

```conf
# Cache hits/misses
ans_cache_hits_total
ans_cache_misses_total

# Cache size
ans_cache_size_bytes
ans_cache_entries_total

# Cache operations
ans_cache_operations_total{operation="get|set|delete"}
```

#### Registry Metrics

```conf
# Registry lookup duration
ans_registry_lookup_duration_seconds{registry="godaddy|mock"}

# Registry errors
ans_registry_errors_total{registry="godaddy|mock",error_type="timeout|not_found"}
```

#### Verification Metrics

```conf
# Verification operations
ans_verifier_operations_total{result="verified|unverified|error"}

# Verification duration
ans_verifier_duration_seconds
```

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'ans-resolver'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Kubernetes ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: ans-resolver
  namespace: ans-system
spec:
  selector:
    matchLabels:
      app: ans-resolver
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Health Checks

### Endpoints

```bash
# Liveness probe
curl http://localhost:8080/health
# Response: {"status":"healthy"}

# Readiness probe
curl http://localhost:8080/ready
# Response: {"status":"ready","checks":{"cache":"ok","registry":"ok"}}
```

### Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 2
```

## Logging

### Log Levels

- **DEBUG**: Detailed troubleshooting info
- **INFO**: General operational messages
- **WARN**: Warning conditions
- **ERROR**: Error conditions

### Configuration

```yaml
logging:
  level: info        # debug, info, warn, error
  format: json       # json, text
  output: stdout     # stdout, file
  file: /var/log/ans-resolver.log
```

### Log Structure (JSON)

```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "info",
  "msg": "Resolution successful",
  "ans_name": "mcp://chatbot.conversation.PID-5678.v1.2.3.example.com",
  "version_range": "^1.0.0",
  "selected_version": "1.2.3",
  "duration_ms": 45,
  "cache_hit": true,
  "trace_id": "abc123"
}
```

## Alerting

### Prometheus Alerts

```yaml
groups:
  - name: ans-resolver
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: rate(ans_resolver_requests_total{status="failure"}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} req/s"

      - alert: HighLatency
        expr: histogram_quantile(0.95, ans_resolver_request_duration_seconds_bucket) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "P95 latency is {{ $value }}s"

      - alert: CacheMissRate
        expr: rate(ans_cache_misses_total[5m]) / (rate(ans_cache_hits_total[5m]) + rate(ans_cache_misses_total[5m])) > 0.8
        for: 10m
        labels:
          severity: info
        annotations:
          summary: "High cache miss rate"
          description: "Cache miss rate is {{ $value }}"

      - alert: ServiceDown
        expr: up{job="ans-resolver"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "ANS Resolver is down"
```

### AlertManager Configuration

```yaml
route:
  receiver: 'team'
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h

receivers:
  - name: 'team'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/xxx'
        channel: '#alerts'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}\n{{ end }}'
```

## Tracing

### OpenTelemetry

```yaml
telemetry:
  tracing:
    enabled: true
    provider: otlp              # otlp (default)
    serviceName: ans-resolver   # Service name in traces
    sampleRate: 0.1             # 10% sampling (0.0-1.0)
    otlp:
      endpoint: otel-collector:4317
      insecure: true            # Use insecure connection (dev only)
      headers: ""               # Optional custom headers
```

### Jaeger Integration

```yaml
telemetry:
  tracing:
    enabled: true
    provider: jaeger
    serviceName: ans-resolver
    sampleRate: 1.0
    jaeger:
      endpoint: jaeger:14268
      agentHost: localhost
      agentPort: 6831
```

View traces at:

- **Jaeger**: http://localhost:16686
- **Zipkin**: http://localhost:9411
- **Grafana Tempo**: via Grafana dashboard

## Debugging

### Debug Endpoints

```bash
# Enable debug logging
curl -X POST http://localhost:8080/debug/log-level?level=debug

# Get pprof profiles
curl http://localhost:8080/debug/pprof/profile > cpu.prof
curl http://localhost:8080/debug/pprof/heap > mem.prof

# Go tool pprof
go tool pprof cpu.prof
```

### Request Tracing

Add trace headers for detailed logging:

```bash
curl -H "X-Trace-ID: abc123" \
     http://localhost:8080/v1/resolve?name=...
```

## Next Steps

- **[Security](security.md)** - Security configuration
- **[Troubleshooting](troubleshooting.md)** - Debug common issues
- **[Deployment](deployment.md)** - Production deployment
