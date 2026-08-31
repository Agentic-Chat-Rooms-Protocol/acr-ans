# Trust Verification Reference

Complete reference for certificate verification and trust management.

## Overview

Trust verification validates agent certificates to ensure secure communication.

## Verification Algorithm

**Step 1: Resolve** - Get endpoint and certificate fingerprint from registry  
**Step 2: Connect** - TLS handshake with agent endpoint  
**Step 3: Extract** - Get peer certificate from connection  
**Step 4: Calculate** - SHA-256 hash of certificate DER bytes  
**Step 5: Compare** - Match calculated fingerprint with registry's fingerprint  
**Step 6: Validate** - Check expiry, revocation (if enabled), and signature  

**Trust anchor:** Agent Registry (similar to SSH host key verification)  
**Protection:** Prevents MITM attacks via certificate pinning

**Current Status:** Implementation complete, disabled by default (awaiting registry support for fingerprints in resolution responses)

## Interface Definition

```go
type Verifier interface {
    // Verify agent certificate matches expected fingerprint
    Verify(ctx context.Context, endpoint, expectedFingerprint string) error
    
    // Validate certificate against trust store
    ValidateCertificate(cert *x509.Certificate) error
    
    // Load trust store from directory
    LoadTrustStore(path string) error
}
```

## Configuration

```yaml
trust:
  enabled: true
  verification_mode: strict  # strict, permissive, disabled
  trust_store: /etc/resolver/certs/trusted
  revocation_check: false  # CRL/OCSP checking
  allow_self_signed: false
  max_cert_age_days: 825  # ~2 years
```

## Verification Modes

### Strict Mode

Full certificate validation:

- Fingerprint match
- Certificate chain validation
- Expiration check
- Revocation check (if enabled)
- Trust store validation

```yaml
trust:
  verification_mode: strict
  trust_store: /etc/resolver/certs/trusted
```

### Permissive Mode

Fingerprint-only validation:

- Fingerprint match only
- No chain validation
- Allows self-signed certificates

```yaml
trust:
  verification_mode: permissive
```

### Disabled Mode

No verification (testing only):

```yaml
trust:
  enabled: false
```

⚠️ **Never use in production!**

## Fingerprint Calculation

### SHA-256 Fingerprint

```go
func CalculateFingerprint(cert *x509.Certificate) string {
    hash := sha256.Sum256(cert.Raw)
    return "SHA256:" + hex.EncodeToString(hash[:])
}
```

### Generate Fingerprint

```bash
# From PEM file
openssl x509 -in cert.pem -noout -fingerprint -sha256

# From endpoint
openssl s_client -connect agent.example.com:8443 2>/dev/null | \
  openssl x509 -noout -fingerprint -sha256
```

## Verification Process

### Step-by-Step

```go
func (v *TrustVerifier) Verify(ctx context.Context, endpoint, expectedFingerprint string) error {
    // 1. Connect to endpoint with TLS
    dialer := &tls.Dialer{
        Config: &tls.Config{
            InsecureSkipVerify: true, // We verify manually
        },
    }
    
    conn, err := dialer.DialContext(ctx, "tcp", endpoint)
    if err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    defer conn.Close()
    
    // 2. Get peer certificates
    tlsConn := conn.(*tls.Conn)
    certs := tlsConn.ConnectionState().PeerCertificates
    if len(certs) == 0 {
        return errors.New("no certificates presented")
    }
    
    // 3. Calculate actual fingerprint
    actualFingerprint := CalculateFingerprint(certs[0])
    
    // 4. Compare fingerprints
    if actualFingerprint != expectedFingerprint {
        return ErrFingerprintMismatch
    }
    
    // 5. Additional validation (strict mode)
    if v.mode == StrictMode {
        return v.ValidateCertificate(certs[0])
    }
    
    return nil
}
```

## Trust Store

### Directory Structure

```
/etc/resolver/certs/trusted/
├── root-ca.pem
├── intermediate-ca.pem
└── agents/
    ├── agent1.pem
    └── agent2.pem
```

### Load Trust Store

```go
func (v *TrustVerifier) LoadTrustStore(path string) error {
    pool := x509.NewCertPool()
    
    files, err := filepath.Glob(filepath.Join(path, "*.pem"))
    if err != nil {
        return err
    }
    
    for _, file := range files {
        pem, err := os.ReadFile(file)
        if err != nil {
            continue
        }
        
        if !pool.AppendCertsFromPEM(pem) {
            log.Warn("failed to load certificate", "file", file)
        }
    }
    
    v.certPool = pool
    return nil
}
```

### Certificate Validation

```go
func (v *TrustVerifier) ValidateCertificate(cert *x509.Certificate) error {
    // Check expiration
    now := time.Now()
    if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
        return ErrCertificateExpired
    }
    
    // Check max age
    age := now.Sub(cert.NotBefore)
    if age > v.maxCertAge {
        return ErrCertificateTooOld
    }
    
    // Verify against trust store
    opts := x509.VerifyOptions{
        Roots:         v.certPool,
        CurrentTime:   now,
        KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    }
    
    _, err := cert.Verify(opts)
    return err
}
```

## Revocation Checking

### CRL (Certificate Revocation List)

```yaml
trust:
  revocation_check: true
  crl_urls:
    - http://crl.example.com/ca.crl
  crl_cache_ttl: 3600
```

### OCSP (Online Certificate Status Protocol)

```go
func (v *TrustVerifier) CheckOCSP(cert, issuer *x509.Certificate) error {
    if len(cert.OCSPServer) == 0 {
        return errors.New("no OCSP servers")
    }
    
    ocspRequest, err := ocsp.CreateRequest(cert, issuer, nil)
    if err != nil {
        return err
    }
    
    resp, err := http.Post(cert.OCSPServer[0], "application/ocsp-request", 
        bytes.NewReader(ocspRequest))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    ocspResp, err := ocsp.ParseResponse(resp.Body, issuer)
    if err != nil {
        return err
    }
    
    if ocspResp.Status != ocsp.Good {
        return ErrCertificateRevoked
    }
    
    return nil
}
```

## Common Errors

### Fingerprint Mismatch

```
Error: certificate fingerprint mismatch
Expected: SHA256:abcd1234...
Actual:   SHA256:efgh5678...
```

**Solutions:**
- Verify certificate hasn't changed
- Update agent record with new fingerprint
- Check for MITM attack

### Certificate Expired

```
Error: certificate has expired
Not After: 2024-12-31 23:59:59
Current:   2025-01-15 10:30:00
```

**Solutions:**
- Renew agent certificate
- Update registry record
- Check system time

### Certificate Revoked

```
Error: certificate has been revoked
```

**Solutions:**
- Issue new certificate
- Update agent record
- Investigate revocation reason

## Best Practices

### 1. Always Verify in Production

```yaml
trust:
  enabled: true
  verification_mode: strict
```

### 2. Regular Certificate Rotation

```bash
# Renew before expiration
# Target: 30 days before expiry
```

### 3. Monitor Certificate Expiration

```promql
(ans_verifier_cert_expiry_seconds - time()) < (30 * 24 * 3600)
```

### 4. Use Strong Ciphers

```yaml
server:
  tls:
    min_version: "1.3"
    cipher_suites:
      - TLS_AES_256_GCM_SHA384
      - TLS_CHACHA20_POLY1305_SHA256
```

### 5. Maintain Trust Store

- Add only trusted CAs
- Remove expired certificates
- Audit regularly

## Certificate Pinning

For critical agents, pin specific certificates:

```yaml
trust:
  pinned_certificates:
    "mcp://critical-agent.PID-999.v1.0.0.example.com":
      fingerprint: "SHA256:abcd1234..."
      expires: "2025-12-31T23:59:59Z"
```

## Metrics

```
# Verification operations
ans_verifier_operations_total{result="verified|unverified|error"}

# Verification duration
ans_verifier_duration_seconds

# Certificate expiration
ans_verifier_cert_expiry_seconds{agent="..."}
```

## Debugging

### Enable Verbose Logging

```yaml
logging:
  level: debug
```

### Manual Verification

```bash
# Test TLS connection
openssl s_client -connect agent.example.com:8443 -showcerts

# Verify certificate chain
openssl verify -CAfile ca.pem agent.pem

# Check certificate details
openssl x509 -in agent.pem -text -noout
```

### Trust Verification Logs

```json
{
  "level": "debug",
  "msg": "verifying certificate",
  "endpoint": "agent.example.com:8443",
  "expected_fingerprint": "SHA256:abcd...",
  "actual_fingerprint": "SHA256:abcd...",
  "match": true,
  "cert_expiry": "2025-12-31T23:59:59Z"
}
```

## Security Considerations

### 1. Protect Private Keys

- Never commit to version control
- Use secrets management (Vault, etc.)
- Restrict file permissions (600)

### 2. Regular Audits

- Review trust store
- Check certificate expiration
- Monitor verification failures

### 3. Incident Response

If certificate compromised:

1. Revoke certificate immediately
2. Issue new certificate
3. Update all agent records
4. Monitor for unauthorized usage

## Next Steps

- **[Security Guide](../security.md)** - Security best practices
- **[Registry Adapters](registry-adapters.md)** - Registry implementations
- **[Monitoring](../monitoring.md)** - Observability
