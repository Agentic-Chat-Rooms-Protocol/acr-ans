// Package trust provides pluggable trust store and verification implementations.
package trust

import (
	"context"
	"crypto"
	"crypto/x509"
	"time"

	"github.com/route-ans/route-ans/internal/models"
	"github.com/route-ans/route-ans/internal/registry"
)

// Provider defines the interface for trust store implementations.
type Provider interface {
	// GetRootCertificates returns the trusted root CA certificates.
	GetRootCertificates(ctx context.Context) ([]*x509.Certificate, error)

	// GetRegistrarKeys returns the public keys for a specific registrar.
	GetRegistrarKeys(ctx context.Context, registrarID string) ([]crypto.PublicKey, error)

	// GetAllRegistrars returns all known registrar configurations.
	GetAllRegistrars(ctx context.Context) ([]*RegistrarInfo, error)

	// Refresh reloads the trust store from its source.
	Refresh(ctx context.Context) error

	// Close releases any resources held by the provider.
	Close() error

	// Name returns the provider name for logging/metrics.
	Name() string
}

// RegistrarInfo contains information about a trusted registrar
type RegistrarInfo struct {
	// ID is the unique registrar identifier
	ID string `json:"id"`

	// Name is the human-readable name
	Name string `json:"name"`

	// PublicKeys contains the registrar's public keys for signature verification
	PublicKeys []PublicKeyInfo `json:"publicKeys"`

	// BaseURL is the registrar's API base URL
	BaseURL string `json:"baseUrl"`

	// TrustLevel indicates the trust tier (e.g., "bronze", "silver", "gold")
	TrustLevel string `json:"trustLevel"`

	// Enabled indicates if the registrar is currently trusted
	Enabled bool `json:"enabled"`
}

// PublicKeyInfo contains information about a public key
type PublicKeyInfo struct {
	// KeyID is the key identifier
	KeyID string `json:"keyId"`

	// Algorithm is the signing algorithm (e.g., "ES256", "RS256")
	Algorithm string `json:"algorithm"`

	// PublicKeyPEM is the PEM-encoded public key
	PublicKeyPEM string `json:"publicKeyPem"`

	// ValidFrom is when the key becomes valid
	ValidFrom time.Time `json:"validFrom"`

	// ValidUntil is when the key expires
	ValidUntil time.Time `json:"validUntil"`
}

// Verifier defines the interface for cryptographic verification.
type Verifier interface {
	// VerifyRecord performs full verification of a resolution record.
	VerifyRecord(ctx context.Context, record *models.ResolutionRecord) (*VerificationResult, error)

	// VerifyRegistryRecord verifies a record from the registry.
	VerifyRegistryRecord(ctx context.Context, record *registry.Record) (*VerificationResult, error)

	// VerifyMerkleProof verifies a Merkle inclusion proof.
	VerifyMerkleProof(ctx context.Context, proof *registry.MerkleProof, sth *registry.SignedTreeHead) (bool, error)

	// VerifySignature verifies a JWS signature.
	VerifySignature(ctx context.Context, payload []byte, signature string, registrarID string) (bool, error)

	// VerifyCertificate verifies an X.509 certificate chain.
	VerifyCertificate(ctx context.Context, certPEM string) (*CertVerificationResult, error)

	// CheckRevocation checks if a certificate is revoked via OCSP or CRL.
	CheckRevocation(ctx context.Context, cert *x509.Certificate) (*RevocationResult, error)
}

// VerificationResult contains the result of full verification
type VerificationResult struct {
	// Valid indicates if all verification checks passed
	Valid bool `json:"valid"`

	// Status is the overall verification status
	Status VerificationStatus `json:"status"`

	// Checks contains individual check results
	Checks map[string]*CheckResult `json:"checks"`

	// Timestamp is when verification was performed
	Timestamp time.Time `json:"timestamp"`

	// CachedUntil is when this result can be cached until
	CachedUntil time.Time `json:"cachedUntil"`

	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// VerificationStatus represents the overall verification status
type VerificationStatus string

const (
	// StatusVerified indicates successful verification of all checks
	StatusVerified  VerificationStatus = "verified"
	StatusInvalid   VerificationStatus = "invalid"
	StatusRevoked   VerificationStatus = "revoked"
	StatusExpired   VerificationStatus = "expired"
	StatusUntrusted VerificationStatus = "untrusted"
	StatusError     VerificationStatus = "error"
)

// CheckResult contains a single verification check result
type CheckResult struct {
	// Name is the check name
	Name string `json:"name"`

	// Passed indicates if the check passed
	Passed bool `json:"passed"`

	// Required indicates if this check must pass
	Required bool `json:"required"`

	// Message describes the result
	Message string `json:"message"`

	// Details contains additional details
	Details map[string]interface{} `json:"details,omitempty"`

	// Duration is how long the check took
	Duration time.Duration `json:"duration"`
}

// CertVerificationResult contains certificate verification results
type CertVerificationResult struct {
	// Valid indicates if the certificate chain is valid
	Valid bool `json:"valid"`

	// Subject is the certificate subject
	Subject string `json:"subject"`

	// Issuer is the certificate issuer
	Issuer string `json:"issuer"`

	// NotBefore is the certificate validity start
	NotBefore time.Time `json:"notBefore"`

	// NotAfter is the certificate validity end
	NotAfter time.Time `json:"notAfter"`

	// ChainLength is the length of the certificate chain
	ChainLength int `json:"chainLength"`

	// Errors contains any validation errors
	Errors []string `json:"errors,omitempty"`
}

// RevocationResult contains revocation check results
type RevocationResult struct {
	// Revoked indicates if the certificate is revoked
	Revoked bool `json:"revoked"`

	// RevokedAt is when the certificate was revoked (if applicable)
	RevokedAt *time.Time `json:"revokedAt,omitempty"`

	// Reason is the revocation reason (if applicable)
	Reason string `json:"reason,omitempty"`

	// CheckMethod indicates how revocation was checked ("ocsp" or "crl")
	CheckMethod string `json:"checkMethod"`

	// CachedUntil is when this result can be cached until
	CachedUntil time.Time `json:"cachedUntil"`
}

// Options contains common trust provider options
type Options struct {
	// RefreshInterval is how often to refresh the trust store
	RefreshInterval time.Duration

	// OCSPEnabled enables OCSP revocation checking
	OCSPEnabled bool

	// OCSPTimeout is the timeout for OCSP requests
	OCSPTimeout time.Duration

	// CRLEnabled enables CRL revocation checking
	CRLEnabled bool

	// CRLCacheTimeout is how long to cache CRL data
	CRLCacheTimeout time.Duration

	// AllowExpiredGracePeriod allows recently expired certs
	AllowExpiredGracePeriod time.Duration
}

// DefaultOptions returns sensible default trust options
func DefaultOptions() Options {
	return Options{
		RefreshInterval:         1 * time.Hour,
		OCSPEnabled:             true,
		OCSPTimeout:             5 * time.Second,
		CRLEnabled:              true,
		CRLCacheTimeout:         24 * time.Hour,
		AllowExpiredGracePeriod: 5 * time.Minute,
	}
}

// Factory is a function that creates a new trust provider
type Factory func(opts Options, config map[string]interface{}) (Provider, error)

// registry holds registered trust provider factories
var providerRegistry = make(map[string]Factory)

// Register registers a trust provider factory
func Register(name string, factory Factory) {
	providerRegistry[name] = factory
}

// New creates a new trust provider by name
func New(name string, opts Options, config map[string]interface{}) (Provider, error) {
	factory, ok := providerRegistry[name]
	if !ok {
		return nil, &ErrUnknownProvider{Name: name}
	}
	return factory(opts, config)
}

// Available returns the names of all registered trust providers
func Available() []string {
	names := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		names = append(names, name)
	}
	return names
}

// ErrUnknownProvider is returned when an unknown provider is requested
type ErrUnknownProvider struct {
	Name string
}

func (e *ErrUnknownProvider) Error() string {
	return "unknown trust provider: " + e.Name
}

// Check names for verification
const (
	CheckSignature   = "signature"
	CheckMerkleProof = "merkle_proof"
	CheckCertificate = "certificate"
	CheckRevocation  = "revocation"
	CheckExpiry      = "expiry"
	CheckRegistrar   = "registrar"
)
