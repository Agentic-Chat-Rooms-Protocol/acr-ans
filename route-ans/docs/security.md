# Security Guide

Security best practices and configuration for Route ANS Resolver.

## TLS/HTTPS

!!! important "TLS Termination"
    The resolver does **not** handle TLS directly. Deploy behind a reverse proxy (nginx, Traefik, Envoy) or load balancer that handles TLS termination.

### Reverse Proxy with TLS (Recommended)

#### Nginx

```nginx
upstream ans_resolver {
    server localhost:8080;
}

server {
    listen 443 ssl http2;
    server_name resolver.example.com;

    ssl_certificate /etc/letsencrypt/live/resolver.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/resolver.example.com/privkey.pem;
    ssl_protocols TLSv1.3;
    ssl_ciphers 'TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256';
    ssl_prefer_server_ciphers on;

    location / {
        proxy_pass http://ans_resolver;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Traefik (Docker)

```yaml
# docker-compose.yml
services:
  traefik:
    image: traefik:v2.10
    command:
      - "--providers.docker=true"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports:
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./acme.json:/acme.json

  ans-resolver:
    image: ans-resolver:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.resolver.rule=Host(`resolver.example.com`)"
      - "traefik.http.routers.resolver.entrypoints=websecure"
      - "traefik.http.routers.resolver.tls.certresolver=letsencrypt"
    expose:
      - "8080"
```

#### Kubernetes Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ans-resolver
  namespace: ans-system
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
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
                  number: 8080
```

## Authentication

!!! warning "Not Implemented"
    The resolver does **not** have built-in authentication. Implement authentication at the reverse proxy or API gateway level.

### Do You Need Authentication?

For most deployments, **authentication is unnecessary**. The ANS resolver is a directory service (like DNS) - it provides public routing information. Security happens at the agent endpoints:

- **Public Resolver** (default): No authentication needed - anyone can discover ANS names
- **Agent-Level Security**: Agents handle TLS, certificate validation, and access control
- **Rate Limiting**: Prevents abuse without blocking legitimate discovery

**Authentication makes sense when:**
- Running **private/internal ANS infrastructure** with sensitive service information
- Operating a **commercial resolver service** with tiered access

### Reverse Proxy Authentication

**Nginx with Basic Auth:**
```nginx
server {
    location / {
        auth_basic "Resolver API";
        auth_basic_user_file /etc/nginx/.htpasswd;
        proxy_pass http://ans_resolver;
    }
}
```

**Nginx with API Key:**
```nginx
server {
    location / {
        if ($http_x_api_key != "your-secret-key") {
            return 401;
        }
        proxy_pass http://ans_resolver;
    }
}
```

**API Gateway:**
- Use AWS API Gateway, Kong, or Tyk for advanced authentication
- Implement OAuth2, JWT validation, or API key management
- Add rate limiting per client/key

### mTLS (Mutual TLS)

Configure mTLS at the reverse proxy or ingress level:

**Nginx:**
```nginx
server {
    listen 443 ssl;
    ssl_client_certificate /etc/nginx/certs/ca.crt;
    ssl_verify_client on;
    
    location / {
        proxy_pass http://ans_resolver;
        proxy_set_header X-Client-Cert $ssl_client_cert;
    }
}
```

**Kubernetes Ingress (with nginx-ingress):**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/auth-tls-verify-client: "on"
    nginx.ingress.kubernetes.io/auth-tls-secret: "ans-system/client-ca"
spec:
  # ... rest of ingress config
```

## Rate Limiting

### Configuration

```yaml
rateLimit:
  enabled: true
  provider: memory  # memory | redis
  keyExtractor: ip  # ip | api-key | client-cert
  limits:
    anonymous:
      requests: 100
      window: 1m
    authenticated:
      requests: 1000
      window: 1m
```

### Redis-Based Rate Limiting

```yaml
rateLimit:
  enabled: true
  provider: redis
  keyExtractor: ip
  limits:
    anonymous:
      requests: 100
      window: 1m
  redis:
    address: redis:6379
    keyPrefix: "ratelimit:"
```

### Kubernetes Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ans-resolver-rate-limit
  namespace: ans-system
spec:
  podSelector:
    matchLabels:
      app: ans-resolver
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: allowed-namespace
```

## Certificate Verification

### Trust Store Configuration

```yaml
trust:
  enabled: true
  verification_mode: strict  # strict, permissive, disabled
  trust_store: /etc/resolver/certs/trusted
  revocation_check: true
```

### Certificate Fingerprint Validation

The resolver validates agent certificates:

```go
// Verify certificate fingerprint matches
if result.CertFingerprint != expectedFingerprint {
    return errors.New("certificate fingerprint mismatch")
}
```

## Network Security

### Firewall Rules

```bash
# Allow only necessary ports
ufw allow 8080/tcp  # HTTP API
ufw allow 9090/tcp  # Metrics
ufw deny 6379/tcp   # Redis (internal only)

# Allow from specific IPs
ufw allow from 10.0.0.0/8 to any port 8080
```

### Kubernetes Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ans-resolver-netpol
  namespace: ans-system
spec:
  podSelector:
    matchLabels:
      app: ans-resolver
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: redis
      ports:
        - protocol: TCP
          port: 6379
    - to:  # DNS
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
```

## Secrets Management

### Environment Variables (Not Recommended)

```bash
export GODADDY_API_KEY=secret
export GODADDY_SECRET=secret
```

### Kubernetes Secrets

```bash
kubectl create secret generic resolver-secrets \
  --from-literal=godaddy-api-key=xxx \
  --from-literal=godaddy-secret=yyy \
  -n ans-system
```

Mount in deployment:

```yaml
env:
  - name: GODADDY_API_KEY
    valueFrom:
      secretKeyRef:
        name: resolver-secrets
        key: godaddy-api-key
```

### HashiCorp Vault

```yaml
# Pod annotation for Vault Agent injection
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "ans-resolver"
  vault.hashicorp.com/agent-inject-secret-config: "secret/ans-resolver"
  vault.hashicorp.com/agent-inject-template-config: |
    {{- with secret "secret/ans-resolver" -}}
    registry:
      godaddy:
        api_key: {{ .Data.data.godaddy_api_key }}
        secret: {{ .Data.data.godaddy_secret }}
    {{- end }}
```

### AWS Secrets Manager

```yaml
env:
  - name: GODADDY_API_KEY
    valueFrom:
      secretRef:
        name: ans-resolver-secrets
        key: godaddy-api-key

# External Secrets Operator
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: ans-resolver-secrets
spec:
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: ans-resolver-secrets
  data:
    - secretKey: godaddy-api-key
      remoteRef:
        key: ans/godaddy
        property: api_key
```

## Next Steps

- **[Monitoring](monitoring.md)** - Security monitoring
- **[Troubleshooting](troubleshooting.md)** - Debug security issues
- **[Deployment](deployment.md)** - Secure deployment

