# Deployment Guide

Production deployment strategies for Route ANS Resolver.

## Docker Deployment

### Basic Container

```bash
# Pull image
docker pull ghcr.io/route-ans/route-ans:latest

# Run with default config
docker run -d \
  --name ans-resolver \
  -p 8080:8080 \
  ghcr.io/route-ans/route-ans:latest

# Run with custom config
docker run -d \
  --name ans-resolver \
  -p 8080:8080 \
  -v /path/to/config.yaml:/etc/resolver/config.yaml \
  ghcr.io/route-ans/route-ans:latest \
  --config /etc/resolver/config.yaml
```

### Docker Compose

```yaml
version: '3.8'

services:
  resolver:
    image: ghcr.io/route-ans/route-ans:latest
    ports:
      - "8080:8080"
    volumes:
      - ./configs/resolver.yaml:/etc/resolver/config.yaml:ro
      - ./certs:/etc/resolver/certs:ro
    environment:
      - ANS_LOG_LEVEL=info
      - ANS_LOG_FORMAT=json
    command: ["--config", "/etc/resolver/config.yaml"]
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    restart: unless-stopped

volumes:
  redis-data:
```

### With Redis Cache

```yaml
version: '3.8'

services:
  resolver:
    image: ghcr.io/route-ans/route-ans:latest
    depends_on:
      - redis
    ports:
      - "8080:8080"
    volumes:
      - ./configs/resolver-redis.yaml:/etc/resolver/config.yaml:ro
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    restart: unless-stopped

volumes:
  redis-data:
```

## Kubernetes Deployment

### Basic Deployment

See [deployments/kubernetes/](https://github.com/route-ans/route-ans/tree/main/deployments/kubernetes/) for complete manifests.

```bash
# Apply all manifests
kubectl apply -k deployments/kubernetes/

# Check status
kubectl get pods -n ans-system
kubectl get svc -n ans-system
```

### Production Setup

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ans-resolver
  namespace: ans-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ans-resolver
  template:
    metadata:
      labels:
        app: ans-resolver
    spec:
      containers:
      - name: resolver
        image: ghcr.io/route-ans/route-ans:v1.0.0
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: ANS_LOG_FORMAT
          value: "json"
        - name: REDIS_HOST
          value: "redis-service"
        volumeMounts:
        - name: config
          mountPath: /etc/resolver
          readOnly: true
        - name: certs
          mountPath: /etc/resolver/certs
          readOnly: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: resolver-config
      - name: certs
        secret:
          secretName: resolver-certs
---
apiVersion: v1
kind: Service
metadata:
  name: ans-resolver
  namespace: ans-system
spec:
  type: LoadBalancer
  selector:
    app: ans-resolver
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
```

### Horizontal Pod Autoscaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ans-resolver-hpa
  namespace: ans-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ans-resolver
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ans-resolver-ingress
  namespace: ans-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - resolver.example.com
    secretName: resolver-tls
  rules:
  - host: resolver.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ans-resolver
            port:
              number: 80
```

## Systemd Service

For bare metal or VM deployments:

```ini
[Unit]
Description=Route ANS Resolver
After=network.target

[Service]
Type=simple
User=ans
Group=ans
ExecStart=/usr/local/bin/ans-resolver --config /etc/ans-resolver/config.yaml
Restart=on-failure
RestartSec=5s

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ans-resolver

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ans-resolver

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ans-resolver
sudo systemctl start ans-resolver
sudo systemctl status ans-resolver
```

## Cloud Deployments

### AWS ECS

```json
{
  "family": "ans-resolver",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "256",
  "memory": "512",
  "containerDefinitions": [
    {
      "name": "resolver",
      "image": "ghcr.io/route-ans/route-ans:latest",
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {"name": "ANS_LOG_FORMAT", "value": "json"}
      ],
      "secrets": [
        {
          "name": "GODADDY_API_KEY",
          "valueFrom": "arn:aws:secretsmanager:region:account:secret:ans/godaddy"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/ans-resolver",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "healthCheck": {
        "command": ["CMD-SHELL", "wget -q -O - http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3
      }
    }
  ]
}
```

### Google Cloud Run

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: ans-resolver
spec:
  template:
    spec:
      containers:
      - image: ghcr.io/route-ans/route-ans:latest
        ports:
        - containerPort: 8080
        env:
        - name: ANS_LOG_FORMAT
          value: json
        resources:
          limits:
            memory: 512Mi
            cpu: 1000m
```

Deploy:

```bash
gcloud run deploy ans-resolver \
  --image ghcr.io/route-ans/route-ans:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --cpu 1
```

### Azure Container Instances

```bash
az container create \
  --resource-group ans-resolver \
  --name ans-resolver \
  --image ghcr.io/route-ans/route-ans:latest \
  --dns-name-label ans-resolver \
  --ports 8080 \
  --cpu 1 \
  --memory 1 \
  --environment-variables ANS_LOG_FORMAT=json
```

## High Availability

### Load Balancing

```nginx
upstream ans_resolvers {
    least_conn;
    server resolver1.internal:8080 max_fails=3 fail_timeout=30s;
    server resolver2.internal:8080 max_fails=3 fail_timeout=30s;
    server resolver3.internal:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    server_name resolver.example.com;

    location / {
        proxy_pass http://ans_resolvers;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        # Health check
        proxy_next_upstream error timeout http_502 http_503;
    }
}
```

### Multi-Region Setup

Deploy to multiple regions with DNS-based routing:

```yaml
# Region 1: us-east-1
resolver-us-east:
  endpoint: resolver-us-east.example.com
  
# Region 2: eu-west-1  
resolver-eu-west:
  endpoint: resolver-eu-west.example.com

# Global
resolver:
  endpoint: resolver.example.com
  routing: geoproximity
```

## Configuration Management

### Environment Variables

Override config with environment variables:

```bash
export ANS_SERVER_PORT=8080
export ANS_LOG_LEVEL=info
export ANS_CACHE_TYPE=redis
export ANS_CACHE_REDIS_HOST=redis:6379
export ANS_REGISTRY_TYPE=godaddy
export ANS_REGISTRY_GODADDY_API_KEY=secret
```

### Secrets Management

#### Kubernetes Secrets

```bash
kubectl create secret generic resolver-secrets \
  --from-literal=godaddy-api-key=xxx \
  --from-literal=godaddy-secret=yyy \
  -n ans-system
```

#### HashiCorp Vault

```yaml
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "ans-resolver"
  vault.hashicorp.com/agent-inject-secret-godaddy: "secret/data/ans/godaddy"
```

## Backup and Recovery

### Configuration Backup

```bash
# Backup config
kubectl get configmap resolver-config -n ans-system -o yaml > backup.yaml

# Restore config
kubectl apply -f backup.yaml
```

### State Backup (Redis)

```bash
# Backup Redis data
docker exec redis redis-cli BGSAVE
docker cp redis:/data/dump.rdb ./backup/

# Restore
docker cp ./backup/dump.rdb redis:/data/
docker restart redis
```

## Rolling Updates

### Zero-Downtime Deployment

```bash
# Update image
kubectl set image deployment/ans-resolver \
  resolver=ghcr.io/route-ans/route-ans:v1.1.0 \
  -n ans-system

# Watch rollout
kubectl rollout status deployment/ans-resolver -n ans-system

# Rollback if needed
kubectl rollout undo deployment/ans-resolver -n ans-system
```

## Next Steps

- **[Monitoring](monitoring.md)** - Metrics and observability
- **[Security](security.md)** - Security best practices
- **[Troubleshooting](troubleshooting.md)** - Common issues
