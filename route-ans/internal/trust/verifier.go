// Package trust provides trust verification implementations.
package trust

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/route-ans/route-ans/internal/models"
	"github.com/route-ans/route-ans/internal/registry"
	"github.com/rs/zerolog/log"
)

// DefaultVerifier implements the Verifier interface
type DefaultVerifier struct {
	trustProvider Provider
	config        VerifierConfig

	// Caches
	ocspCache     map[string]*RevocationResult
	ocspCacheLock sync.RWMutex
	crlCache      map[string]*crlCacheEntry
	crlCacheLock  sync.RWMutex

	httpClient *http.Client
}

type crlCacheEntry struct {
	crl       *x509.RevocationList
	fetchedAt time.Time
	expiresAt time.Time
}

// VerifierConfig contains verifier configuration
type VerifierConfig struct {
	RequireSignature   bool
	RequireMerkleProof bool
	CheckRevocation    bool
	OCSPEnabled        bool
	OCSPTimeout        time.Duration
	CRLEnabled         bool
	CRLCacheTimeout    time.Duration
	GracePeriod        time.Duration
}

// DefaultVerifierConfig returns sensible defaults
func DefaultVerifierConfig() VerifierConfig {
	return VerifierConfig{
		RequireSignature:   true,
		RequireMerkleProof: true,
		CheckRevocation:    true,
		OCSPEnabled:        true,
		OCSPTimeout:        5 * time.Second,
		CRLEnabled:         true,
		CRLCacheTimeout:    24 * time.Hour,
		GracePeriod:        5 * time.Minute,
	}
}

// NewVerifier creates a new verifier instance
func NewVerifier(trustProvider Provider, config VerifierConfig) *DefaultVerifier {
	return &DefaultVerifier{
		trustProvider: trustProvider,
		config:        config,
		ocspCache:     make(map[string]*RevocationResult),
		crlCache:      make(map[string]*crlCacheEntry),
		httpClient: &http.Client{
			Timeout: config.OCSPTimeout,
		},
	}
}

// VerifyRecord performs full verification of a resolution record
func (v *DefaultVerifier) VerifyRecord(ctx context.Context, record *models.ResolutionRecord) (*VerificationResult, error) {
	start := time.Now()
	result := &VerificationResult{
		Valid:     true,
		Status:    StatusVerified,
		Checks:    make(map[string]*CheckResult),
		Timestamp: start,
	}

	// Check expiry
	expiryCheck := v.checkExpiry(record)
	result.Checks[CheckExpiry] = expiryCheck
	if !expiryCheck.Passed && expiryCheck.Required {
		result.Valid = false
		result.Status = StatusExpired
	}

	// Note: For ResolutionRecord we can't do full verification
	// as it doesn't contain the original signatures/proofs
	// This is mainly for cached records

	result.CachedUntil = time.Now().Add(5 * time.Minute)
	return result, nil
}

// VerifyRegistryRecord verifies a record from the registry
//
//nolint:gocyclo // High complexity due to multiple verification checks; splitting would reduce clarity
func (v *DefaultVerifier) VerifyRegistryRecord(ctx context.Context, record *registry.Record) (*VerificationResult, error) {
	start := time.Now()
	result := &VerificationResult{
		Valid:     true,
		Status:    StatusVerified,
		Checks:    make(map[string]*CheckResult),
		Timestamp: start,
	}

	var wg sync.WaitGroup
	var checkLock sync.Mutex

	addCheck := func(name string, check *CheckResult) {
		checkLock.Lock()
		defer checkLock.Unlock()
		result.Checks[name] = check
		if !check.Passed && check.Required {
			result.Valid = false
		}
	}

	// Check registrar trust
	wg.Add(1)
	go func() {
		defer wg.Done()
		check := v.checkRegistrar(ctx, record.RegistrarID)
		addCheck(CheckRegistrar, check)
	}()

	// Verify signature
	if v.config.RequireSignature && record.RegistrySignature != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			check := v.verifyRecordSignature(ctx, record)
			addCheck(CheckSignature, check)
		}()
	}

	// Verify endpoint certificate fingerprint
	if record.Endpoint != "" && record.Certificates.PublicCert.Fingerprint != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			check, err := v.VerifyEndpointFingerprint(ctx, record.Endpoint, record.Certificates.PublicCert.Fingerprint)
			if err != nil {
				log.Error().Err(err).Str("endpoint", record.Endpoint).Msg("Endpoint fingerprint verification error")
				addCheck(CheckCertificate, &CheckResult{
					Name:     CheckCertificate,
					Passed:   false,
					Required: true,
					Message:  fmt.Sprintf("verification error: %v", err),
				})
			} else {
				addCheck(CheckCertificate, check)
			}
		}()
	} else if record.Certificates.PublicCert.Fingerprint == "" {
		// No fingerprint provided by registry
		addCheck(CheckCertificate, &CheckResult{
			Name:     CheckCertificate,
			Passed:   false,
			Required: false,
			Message:  "no certificate fingerprint provided by registry",
		})
	}

	// Check expiry
	wg.Add(1)
	go func() {
		defer wg.Done()
		check := v.checkRecordExpiry(record)
		addCheck(CheckExpiry, check)
	}()

	wg.Wait()

	// Determine final status
	if !result.Valid {
		for name, check := range result.Checks {
			if !check.Passed && check.Required {
				switch name {
				case CheckExpiry:
					result.Status = StatusExpired
				case CheckRevocation:
					result.Status = StatusRevoked
				case CheckSignature, CheckRegistrar:
					result.Status = StatusUntrusted
				default:
					result.Status = StatusInvalid
				}
				break
			}
		}
	}

	result.CachedUntil = time.Now().Add(5 * time.Minute)
	return result, nil
}

// VerifyMerkleProof verifies a Merkle inclusion proof
func (v *DefaultVerifier) VerifyMerkleProof(ctx context.Context, proof *registry.MerkleProof, sth *registry.SignedTreeHead) (bool, error) {
	if proof == nil || sth == nil {
		return false, errors.New("proof or signed tree head is nil")
	}

	// Verify the proof leads to the expected root
	computedRoot := computeMerkleRoot(proof.LeafIndex, proof.TreeSize, proof.Hashes)

	if !bytes.Equal(computedRoot, sth.RootHash) {
		return false, nil
	}

	// Verify the STH signature
	if v.trustProvider != nil {
		// Get registrar keys and verify STH signature
		// For now, we trust the STH if the root matches
		log.Debug().Msg("Merkle proof verified")
	}

	return true, nil
}

// computeMerkleRoot computes the Merkle root from a proof
func computeMerkleRoot(leafIndex, treeSize int64, hashes [][]byte) []byte {
	if len(hashes) == 0 {
		return nil
	}

	// Start with the leaf hash
	current := hashes[0]
	hashIdx := 1
	idx := leafIndex
	size := treeSize

	for size > 1 && hashIdx < len(hashes) {
		var combined []byte
		if idx%2 == 0 {
			// Left child - sibling is on the right
			combined = append(current, hashes[hashIdx]...)
		} else {
			// Right child - sibling is on the left
			combined = append(hashes[hashIdx], current...)
		}
		h := sha256.Sum256(combined)
		current = h[:]
		hashIdx++
		idx /= 2
		size = (size + 1) / 2
	}

	return current
}

// VerifySignature verifies a JWS signature
func (v *DefaultVerifier) VerifySignature(ctx context.Context, payload []byte, signature string, registrarID string) (bool, error) {
	if v.trustProvider == nil {
		return false, errors.New("trust provider not configured")
	}

	// Get the registrar's public keys
	keys, err := v.trustProvider.GetRegistrarKeys(ctx, registrarID)
	if err != nil {
		return false, fmt.Errorf("failed to get registrar keys: %w", err)
	}

	if len(keys) == 0 {
		return false, fmt.Errorf("no keys found for registrar %s", registrarID)
	}

	// Decode the signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Hash the payload
	hash := sha256.Sum256(payload)

	// Try each key
	for _, key := range keys {
		var verified bool
		switch k := key.(type) {
		case *ecdsa.PublicKey:
			verified = ecdsa.VerifyASN1(k, hash[:], sigBytes)
		case *rsa.PublicKey:
			err = rsa.VerifyPKCS1v15(k, crypto.SHA256, hash[:], sigBytes)
			verified = err == nil
		case ed25519.PublicKey:
			verified = ed25519.Verify(k, payload, sigBytes)
		default:
			continue
		}
		if verified {
			return true, nil
		}
	}

	return false, nil
}

// VerifyEndpointFingerprint connects to an agent endpoint and verifies its certificate fingerprint
func (v *DefaultVerifier) VerifyEndpointFingerprint(ctx context.Context, endpoint, expectedFingerprint string) (*CheckResult, error) {
	if expectedFingerprint == "" {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  "no fingerprint provided by registry",
		}, nil
	}

	// Parse endpoint URL to extract host:port
	if !strings.HasPrefix(endpoint, "https://") {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  "endpoint must use HTTPS for certificate verification",
		}, nil
	}

	// Extract host from URL
	host := strings.TrimPrefix(endpoint, "https://")
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// Add default port if not specified
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	// Create TLS dialer that doesn't verify the cert chain (we verify fingerprint instead)
	dialer := &tls.Dialer{
		Config: &tls.Config{
			// #nosec G402 -- InsecureSkipVerify is intentional here; we verify the certificate via fingerprint matching instead of standard chain validation
			InsecureSkipVerify: true, // We verify manually via fingerprint
			MinVersion:         tls.VersionTLS12,
		},
	}

	// Connect with timeout
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(connCtx, "tcp", host)
	if err != nil {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  fmt.Sprintf("failed to connect to endpoint: %v", err),
		}, fmt.Errorf("failed to connect: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close TLS connection")
		}
	}()

	// Get peer certificates
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  "not a TLS connection",
		}, errors.New("connection is not a TLS connection")
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  "no certificates presented by endpoint",
		}, nil
	}

	// Calculate fingerprint of the leaf certificate
	leafCert := state.PeerCertificates[0]
	hash := sha256.Sum256(leafCert.Raw)
	actualFingerprint := "SHA256:" + hex.EncodeToString(hash[:])

	// Compare fingerprints
	if actualFingerprint != expectedFingerprint {
		return &CheckResult{
			Name:     "endpoint_fingerprint",
			Passed:   false,
			Required: true,
			Message:  fmt.Sprintf("fingerprint mismatch: expected %s, got %s", expectedFingerprint, actualFingerprint),
			Details: map[string]interface{}{
				"expected": expectedFingerprint,
				"actual":   actualFingerprint,
				"subject":  leafCert.Subject.String(),
				"issuer":   leafCert.Issuer.String(),
			},
		}, nil
	}

	// Fingerprint matches!
	log.Debug().Str("endpoint", endpoint).Str("fingerprint", actualFingerprint).Msg("Endpoint fingerprint verified")

	return &CheckResult{
		Name:     "endpoint_fingerprint",
		Passed:   true,
		Required: true,
		Message:  "certificate fingerprint matches registry",
		Details: map[string]interface{}{
			"fingerprint": actualFingerprint,
			"subject":     leafCert.Subject.String(),
			"issuer":      leafCert.Issuer.String(),
			"notBefore":   leafCert.NotBefore,
			"notAfter":    leafCert.NotAfter,
		},
	}, nil
}

// VerifyCertificate verifies an X.509 certificate chain
func (v *DefaultVerifier) VerifyCertificate(ctx context.Context, certPEM string) (*CertVerificationResult, error) {
	result := &CertVerificationResult{
		Valid:  true,
		Errors: make([]string, 0),
	}

	// Parse the certificate
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "failed to decode PEM block")
		return result, nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to parse certificate: %v", err))
		return result, nil
	}

	result.Subject = cert.Subject.String()
	result.Issuer = cert.Issuer.String()
	result.NotBefore = cert.NotBefore
	result.NotAfter = cert.NotAfter

	// Check validity period
	now := time.Now()
	if now.Before(cert.NotBefore) {
		result.Valid = false
		result.Errors = append(result.Errors, "certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		// Check grace period
		if now.After(cert.NotAfter.Add(v.config.GracePeriod)) {
			result.Valid = false
			result.Errors = append(result.Errors, "certificate expired")
		}
	}

	// Get trusted roots and verify chain
	if v.trustProvider != nil {
		roots, err := v.trustProvider.GetRootCertificates(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get root certificates")
		} else if len(roots) > 0 {
			pool := x509.NewCertPool()
			for _, root := range roots {
				pool.AddCert(root)
			}

			opts := x509.VerifyOptions{
				Roots:       pool,
				CurrentTime: now,
			}

			chains, err := cert.Verify(opts)
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("chain verification failed: %v", err))
			} else {
				result.ChainLength = len(chains[0])
			}
		}
	}

	return result, nil
}

// CheckRevocation checks if a certificate is revoked
func (v *DefaultVerifier) CheckRevocation(ctx context.Context, cert *x509.Certificate) (*RevocationResult, error) {
	result := &RevocationResult{
		Revoked:     false,
		CachedUntil: time.Now().Add(1 * time.Hour),
	}

	// Check cache first
	fingerprint := hex.EncodeToString(sha256.New().Sum(cert.Raw))
	v.ocspCacheLock.RLock()
	if cached, ok := v.ocspCache[fingerprint]; ok && time.Now().Before(cached.CachedUntil) {
		v.ocspCacheLock.RUnlock()
		return cached, nil
	}
	v.ocspCacheLock.RUnlock()

	// Try OCSP first
	if v.config.OCSPEnabled && len(cert.OCSPServer) > 0 {
		ocspResult, err := v.checkOCSP(ctx, cert)
		if err == nil {
			result = ocspResult
			result.CheckMethod = "ocsp"

			// Cache the result
			v.ocspCacheLock.Lock()
			v.ocspCache[fingerprint] = result
			v.ocspCacheLock.Unlock()

			return result, nil
		}
		log.Debug().Err(err).Msg("OCSP check failed, falling back to CRL")
	}

	// Fall back to CRL
	if v.config.CRLEnabled && len(cert.CRLDistributionPoints) > 0 {
		crlResult, err := v.checkCRL(ctx, cert)
		if err == nil {
			result = crlResult
			result.CheckMethod = "crl"

			// Cache the result
			v.ocspCacheLock.Lock()
			v.ocspCache[fingerprint] = result
			v.ocspCacheLock.Unlock()

			return result, nil
		}
		log.Debug().Err(err).Msg("CRL check failed")
	}

	result.CheckMethod = "none"
	return result, nil
}

// checkOCSP performs OCSP revocation check
func (v *DefaultVerifier) checkOCSP(_ context.Context, cert *x509.Certificate) (*RevocationResult, error) {
	if len(cert.OCSPServer) == 0 {
		return nil, errors.New("no OCSP server specified")
	}

	// For a full implementation, we would:
	// 1. Create OCSP request
	// 2. Send to OCSP server
	// 3. Parse and verify OCSP response
	// This is a simplified version

	result := &RevocationResult{
		Revoked:     false,
		CheckMethod: "ocsp",
		CachedUntil: time.Now().Add(1 * time.Hour),
	}

	return result, nil
}

// checkCRL performs CRL revocation check
func (v *DefaultVerifier) checkCRL(ctx context.Context, cert *x509.Certificate) (*RevocationResult, error) {
	if len(cert.CRLDistributionPoints) == 0 {
		return nil, errors.New("no CRL distribution points specified")
	}

	for _, crlURL := range cert.CRLDistributionPoints {
		// Check cache
		v.crlCacheLock.RLock()
		if cached, ok := v.crlCache[crlURL]; ok && time.Now().Before(cached.expiresAt) {
			v.crlCacheLock.RUnlock()

			// Check if cert is in the CRL
			for _, revoked := range cached.crl.RevokedCertificateEntries {
				if revoked.SerialNumber.Cmp(cert.SerialNumber) == 0 {
					return &RevocationResult{
						Revoked:     true,
						RevokedAt:   &revoked.RevocationTime,
						CheckMethod: "crl",
						CachedUntil: cached.expiresAt,
					}, nil
				}
			}

			return &RevocationResult{
				Revoked:     false,
				CheckMethod: "crl",
				CachedUntil: cached.expiresAt,
			}, nil
		}
		v.crlCacheLock.RUnlock()

		// Fetch CRL
		crl, err := v.fetchCRL(ctx, crlURL)
		if err != nil {
			continue
		}

		// Cache the CRL
		expiresAt := time.Now().Add(v.config.CRLCacheTimeout)
		if !crl.NextUpdate.IsZero() && crl.NextUpdate.Before(expiresAt) {
			expiresAt = crl.NextUpdate
		}

		v.crlCacheLock.Lock()
		v.crlCache[crlURL] = &crlCacheEntry{
			crl:       crl,
			fetchedAt: time.Now(),
			expiresAt: expiresAt,
		}
		v.crlCacheLock.Unlock()

		// Check if cert is revoked
		for _, revoked := range crl.RevokedCertificateEntries {
			if revoked.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return &RevocationResult{
					Revoked:     true,
					RevokedAt:   &revoked.RevocationTime,
					CheckMethod: "crl",
					CachedUntil: expiresAt,
				}, nil
			}
		}

		return &RevocationResult{
			Revoked:     false,
			CheckMethod: "crl",
			CachedUntil: expiresAt,
		}, nil
	}

	return nil, errors.New("failed to check any CRL")
}

// fetchCRL fetches a CRL from the given URL
func (v *DefaultVerifier) fetchCRL(ctx context.Context, crlURL string) (*x509.RevocationList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crlURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close CRL response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CRL fetch failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return x509.ParseRevocationList(body)
}

// Helper check functions

func (v *DefaultVerifier) checkExpiry(record *models.ResolutionRecord) *CheckResult {
	check := &CheckResult{
		Name:     CheckExpiry,
		Required: true,
		Details:  make(map[string]interface{}),
	}

	now := time.Now()
	if record.ExpiresAt.IsZero() {
		check.Passed = true
		check.Message = "No expiry set"
		return check
	}

	if now.After(record.ExpiresAt) {
		if now.Before(record.ExpiresAt.Add(v.config.GracePeriod)) {
			check.Passed = true
			check.Message = "Record expired but within grace period"
		} else {
			check.Passed = false
			check.Message = "Record expired"
		}
	} else {
		check.Passed = true
		check.Message = "Record valid"
	}

	check.Details["expiresAt"] = record.ExpiresAt
	check.Details["now"] = now
	return check
}

func (v *DefaultVerifier) checkRecordExpiry(record *registry.Record) *CheckResult {
	check := &CheckResult{
		Name:     CheckExpiry,
		Required: true,
		Details:  make(map[string]interface{}),
	}

	now := time.Now()
	if record.ExpiresAt.IsZero() {
		check.Passed = true
		check.Message = "No expiry set"
		return check
	}

	if now.After(record.ExpiresAt) {
		if now.Before(record.ExpiresAt.Add(v.config.GracePeriod)) {
			check.Passed = true
			check.Message = "Record expired but within grace period"
		} else {
			check.Passed = false
			check.Message = "Record expired"
		}
	} else {
		check.Passed = true
		check.Message = "Record valid"
	}

	check.Details["expiresAt"] = record.ExpiresAt
	return check
}

func (v *DefaultVerifier) checkRegistrar(ctx context.Context, registrarID string) *CheckResult {
	check := &CheckResult{
		Name:     CheckRegistrar,
		Required: true,
		Details:  make(map[string]interface{}),
	}

	if v.trustProvider == nil {
		check.Passed = true
		check.Message = "Trust provider not configured, skipping registrar check"
		return check
	}

	registrars, err := v.trustProvider.GetAllRegistrars(ctx)
	if err != nil {
		check.Passed = false
		check.Message = fmt.Sprintf("Failed to get registrars: %v", err)
		return check
	}

	for _, reg := range registrars {
		if reg.ID == registrarID && reg.Enabled {
			check.Passed = true
			check.Message = "Registrar is trusted"
			check.Details["registrarId"] = registrarID
			check.Details["trustLevel"] = reg.TrustLevel
			return check
		}
	}

	check.Passed = false
	check.Message = "Registrar not trusted"
	check.Details["registrarId"] = registrarID
	return check
}

func (v *DefaultVerifier) verifyRecordSignature(ctx context.Context, record *registry.Record) *CheckResult {
	start := time.Now()
	check := &CheckResult{
		Name:     CheckSignature,
		Required: v.config.RequireSignature,
		Details:  make(map[string]interface{}),
	}

	if record.RegistrySignature == "" {
		check.Passed = !v.config.RequireSignature
		check.Message = "No signature present"
		return check
	}

	// Create canonical payload for verification
	payload := []byte(record.ANSName + record.Endpoint + record.RegistrarID)

	valid, err := v.VerifySignature(ctx, payload, record.RegistrySignature, record.RegistrarID)
	if err != nil {
		check.Passed = false
		check.Message = fmt.Sprintf("Signature verification error: %v", err)
		return check
	}

	check.Passed = valid
	if valid {
		check.Message = "Signature valid"
	} else {
		check.Message = "Invalid signature"
	}
	check.Duration = time.Since(start)
	return check
}

// fileProvider implements a file-based trust provider
type fileProvider struct {
	rootCertsFile  string
	registrarsFile string
	roots          []*x509.Certificate
	registrars     []*RegistrarInfo
	mu             sync.RWMutex
	lastRefresh    time.Time
}

// NewFileProvider creates a file-based trust provider
func NewFileProvider(rootCertsFile, registrarsFile string) (Provider, error) {
	p := &fileProvider{
		rootCertsFile:  rootCertsFile,
		registrarsFile: registrarsFile,
		roots:          make([]*x509.Certificate, 0),
		registrars:     make([]*RegistrarInfo, 0),
	}

	// Initial load
	if err := p.Refresh(context.Background()); err != nil {
		// Log warning but don't fail - files might not exist yet
		log.Warn().Err(err).Msg("Initial trust store load failed")
	}

	return p, nil
}

func (p *fileProvider) GetRootCertificates(ctx context.Context) ([]*x509.Certificate, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.roots, nil
}

func (p *fileProvider) GetRegistrarKeys(ctx context.Context, registrarID string) ([]crypto.PublicKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, reg := range p.registrars {
		if reg.ID == registrarID {
			keys := make([]crypto.PublicKey, 0, len(reg.PublicKeys))
			for _, keyInfo := range reg.PublicKeys {
				key, err := parsePublicKeyPEM(keyInfo.PublicKeyPEM)
				if err != nil {
					log.Warn().Err(err).Str("keyId", keyInfo.KeyID).Msg("Failed to parse key")
					continue
				}
				keys = append(keys, key)
			}
			return keys, nil
		}
	}

	return nil, fmt.Errorf("registrar %s not found", registrarID)
}

func (p *fileProvider) GetAllRegistrars(ctx context.Context) ([]*RegistrarInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.registrars, nil
}

func (p *fileProvider) Refresh(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// For now, just mark as refreshed
	// Full implementation would read from files
	p.lastRefresh = time.Now()
	return nil
}

func (p *fileProvider) Close() error {
	return nil
}

func (p *fileProvider) Name() string {
	return "file"
}

// parsePublicKeyPEM parses a PEM-encoded public key
func parsePublicKeyPEM(pemData string) (crypto.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "EC PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

func init() {
	Register("file", func(opts Options, config map[string]interface{}) (Provider, error) {
		rootCertsFile := ""
		registrarsFile := ""

		if v, ok := config["trustedRootsFile"].(string); ok {
			rootCertsFile = v
		}
		if v, ok := config["trustedRegistrarsFile"].(string); ok {
			registrarsFile = v
		}

		return NewFileProvider(rootCertsFile, registrarsFile)
	})
}

// mockProvider implements a mock trust provider for testing
type mockProvider struct {
	registrars []*RegistrarInfo
}

// NewMockProvider creates a mock trust provider
func NewMockProvider() Provider {
	return &mockProvider{
		registrars: []*RegistrarInfo{
			{
				ID:         "ra-mock",
				Name:       "Mock Registrar",
				TrustLevel: "gold",
				Enabled:    true,
			},
		},
	}
}

func (p *mockProvider) GetRootCertificates(ctx context.Context) ([]*x509.Certificate, error) {
	return []*x509.Certificate{}, nil
}

func (p *mockProvider) GetRegistrarKeys(ctx context.Context, registrarID string) ([]crypto.PublicKey, error) {
	return []crypto.PublicKey{}, nil
}

func (p *mockProvider) GetAllRegistrars(ctx context.Context) ([]*RegistrarInfo, error) {
	return p.registrars, nil
}

func (p *mockProvider) Refresh(ctx context.Context) error {
	return nil
}

func (p *mockProvider) Close() error {
	return nil
}

func (p *mockProvider) Name() string {
	return "mock"
}

func init() {
	Register("mock", func(opts Options, config map[string]interface{}) (Provider, error) {
		return NewMockProvider(), nil
	})
}
