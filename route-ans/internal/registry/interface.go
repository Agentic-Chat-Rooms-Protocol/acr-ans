// Package registry provides pluggable registry adapter implementations.
package registry

import (
	"context"
	"time"

	"github.com/route-ans/route-ans/internal/models"
	"github.com/route-ans/route-ans/pkg/ansname"
)

// Adapter defines the interface for registry backend implementations.
// Each adapter handles communication with a specific type of ANS Registry.
type Adapter interface {
	// Lookup queries the registry for an agent by ANSName.
	// Returns the registry record if found, or nil if not found.
	Lookup(ctx context.Context, name *ansname.ANSName) (*Record, error)

	// LookupByFQDN queries the registry for all versions of an agent by FQDN.
	LookupByFQDN(ctx context.Context, fqdn string) ([]*Record, error)

	// GetMerkleProof retrieves the Merkle inclusion proof for verification.
	GetMerkleProof(ctx context.Context, ansName string) (*MerkleProof, error)

	// GetSignedTreeHead retrieves the current signed tree head for verification.
	GetSignedTreeHead(ctx context.Context) (*SignedTreeHead, error)

	// VerifyRecord verifies a record's authenticity using the registry's proofs.
	VerifyRecord(ctx context.Context, record *Record) (*VerificationResult, error)

	// Subscribe starts receiving events from the registry.
	// This is optional - not all registries may support event streaming.
	Subscribe(ctx context.Context, handler EventHandler) error

	// Name returns the adapter name for logging/metrics.
	Name() string

	// Priority returns the adapter priority (lower = higher priority).
	Priority() int

	// Healthy checks if the registry is reachable and healthy.
	Healthy(ctx context.Context) (bool, error)

	// Close releases any resources held by the adapter.
	Close() error
}

// Record represents a registry record returned by an adapter
type Record struct {
	// ANSName is the full ANSName identifier
	ANSName string `json:"ansName"`

	// FQDN is the stable fully qualified domain name
	FQDN string `json:"fqdn"`

	// Protocol is the agent's primary protocol
	Protocol string `json:"protocol"`

	// Version is the agent version
	Version string `json:"version"`

	// Status is the agent status (active, deprecated, revoked)
	Status string `json:"status"`

	// Endpoint is the agent's service endpoint URL
	Endpoint string `json:"endpoint"`

	// Certificates contains certificate information
	Certificates CertificateInfo `json:"certificates"`

	// Metadata contains agent metadata
	Metadata AgentMetadata `json:"metadata"`

	// RegistrySignature is the registry's signature on this record
	RegistrySignature string `json:"registrySignature"`

	// RegistrarID identifies the issuing registrar
	RegistrarID string `json:"registrarId"`

	// TTL is the cache time-to-live for this record
	TTL time.Duration `json:"ttl"`

	// RegisteredAt is when the agent was first registered
	RegisteredAt time.Time `json:"registeredAt"`

	// UpdatedAt is when the record was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// ExpiresAt is when the record/certificate expires
	ExpiresAt time.Time `json:"expiresAt"`
}

// CertificateInfo contains certificate details
type CertificateInfo struct {
	// PublicCert contains public server certificate info
	PublicCert CertDetails `json:"public"`

	// PrivateCert contains private identity certificate info
	PrivateCert CertDetails `json:"private"`
}

// CertDetails contains details about a specific certificate
type CertDetails struct {
	// Fingerprint is the SHA-256 fingerprint of the certificate
	Fingerprint string `json:"fingerprint"`

	// IssuedAt is when the certificate was issued
	IssuedAt time.Time `json:"issuedAt"`

	// ExpiresAt is when the certificate expires
	ExpiresAt time.Time `json:"expiresAt"`

	// Issuer is the certificate issuer
	Issuer string `json:"issuer"`

	// SerialNumber is the certificate serial number
	SerialNumber string `json:"serialNumber"`
}

// AgentMetadata contains agent descriptive metadata
type AgentMetadata struct {
	// DisplayName is a human-readable name
	DisplayName string `json:"displayName"`

	// Description describes the agent's purpose
	Description string `json:"description"`

	// Capabilities lists the agent's capabilities
	Capabilities []string `json:"capabilities"`

	// Tags are searchable tags
	Tags []string `json:"tags"`

	// Protocols lists all supported protocols
	Protocols []string `json:"protocols"`

	// ProtocolExtensions contains protocol-specific metadata
	ProtocolExtensions map[string]interface{} `json:"protocolExtensions"`

	// AgentCardURL is the URL to the full agent card
	AgentCardURL string `json:"agentCardUrl"`

	// AgentCardHash is the hash of the agent card content
	AgentCardHash string `json:"agentCardHash"`

	// Organization is the owning organization
	Organization string `json:"organization"`

	// OrganizationDomain is the organization's domain
	OrganizationDomain string `json:"organizationDomain"`
}

// MerkleProof contains a Merkle inclusion proof
type MerkleProof struct {
	// LeafIndex is the index of the leaf in the tree
	LeafIndex int64 `json:"leafIndex"`

	// TreeSize is the size of the tree when the proof was generated
	TreeSize int64 `json:"treeSize"`

	// Hashes is the list of sibling hashes for verification
	Hashes [][]byte `json:"hashes"`

	// RootHash is the expected root hash
	RootHash []byte `json:"rootHash"`

	// Timestamp is when the proof was generated
	Timestamp time.Time `json:"timestamp"`
}

// SignedTreeHead contains a signed Merkle tree head
type SignedTreeHead struct {
	// TreeSize is the number of entries in the tree
	TreeSize int64 `json:"treeSize"`

	// RootHash is the Merkle tree root hash
	RootHash []byte `json:"rootHash"`

	// Timestamp is when the tree head was signed
	Timestamp time.Time `json:"timestamp"`

	// Signature is the KMS signature on the tree head
	Signature []byte `json:"signature"`

	// KeyID identifies the signing key
	KeyID string `json:"keyId"`
}

// VerificationResult contains the result of record verification
type VerificationResult struct {
	// Valid indicates if verification passed
	Valid bool `json:"valid"`

	// Checks contains individual check results
	Checks map[string]CheckResult `json:"checks"`

	// Timestamp is when verification was performed
	Timestamp time.Time `json:"timestamp"`
}

// CheckResult contains a single verification check result
type CheckResult struct {
	// Passed indicates if the check passed
	Passed bool `json:"passed"`

	// Message describes the result
	Message string `json:"message"`

	// Details contains additional details
	Details map[string]interface{} `json:"details,omitempty"`
}

// EventHandler processes events from a registry
type EventHandler func(ctx context.Context, event *Event) error

// Event represents a registry lifecycle event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	ANSName   string                 `json:"ansName"`
	FQDN      string                 `json:"fqdn"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature"`
}

// ToResolutionRecord converts a registry record to a resolution record
func (r *Record) ToResolutionRecord() *models.ResolutionRecord {
	return &models.ResolutionRecord{
		ANSName:         r.ANSName,
		FQDN:            r.FQDN,
		Protocol:        r.Protocol,
		Version:         r.Version,
		Status:          r.Status,
		Endpoint:        r.Endpoint,
		CertFingerprint: r.Certificates.PublicCert.Fingerprint,
		CertExpiresAt:   r.Certificates.PublicCert.ExpiresAt,
		DisplayName:     r.Metadata.DisplayName,
		Description:     r.Metadata.Description,
		Capabilities:    r.Metadata.Capabilities,
		Protocols:       r.Metadata.Protocols,
		RegistrarID:     r.RegistrarID,
		TTL:             r.TTL,
		RegisteredAt:    r.RegisteredAt,
		UpdatedAt:       r.UpdatedAt,
		ExpiresAt:       r.ExpiresAt,
	}
}

// Options contains common registry adapter options
type Options struct {
	// Timeout is the request timeout
	Timeout time.Duration

	// Retries is the number of retry attempts
	Retries int

	// RetryBackoff is the initial retry backoff duration
	RetryBackoff time.Duration

	// Priority is the adapter priority (lower = higher priority)
	Priority int
}

// DefaultOptions returns sensible default adapter options
func DefaultOptions() Options {
	return Options{
		Timeout:      10 * time.Second,
		Retries:      3,
		RetryBackoff: 1 * time.Second,
		Priority:     1,
	}
}

// Factory is a function that creates a new registry adapter
type Factory func(opts Options, config map[string]interface{}) (Adapter, error)

// registry holds registered adapter factories
var adapterRegistry = make(map[string]Factory)

// Register registers a registry adapter factory
func Register(name string, factory Factory) {
	adapterRegistry[name] = factory
}

// New creates a new registry adapter by name
func New(name string, opts Options, config map[string]interface{}) (Adapter, error) {
	factory, ok := adapterRegistry[name]
	if !ok {
		return nil, &ErrUnknownAdapter{Name: name}
	}
	return factory(opts, config)
}

// Available returns the names of all registered adapters
func Available() []string {
	names := make([]string, 0, len(adapterRegistry))
	for name := range adapterRegistry {
		names = append(names, name)
	}
	return names
}

// ErrUnknownAdapter is returned when an unknown adapter is requested
type ErrUnknownAdapter struct {
	Name string
}

func (e *ErrUnknownAdapter) Error() string {
	return "unknown registry adapter: " + e.Name
}

// ErrNotFound is returned when an agent is not found
type ErrNotFound struct {
	ANSName string
}

func (e *ErrNotFound) Error() string {
	return "agent not found: " + e.ANSName
}

// ErrRegistryUnavailable is returned when the registry is unavailable
type ErrRegistryUnavailable struct {
	Name    string
	Message string
}

func (e *ErrRegistryUnavailable) Error() string {
	return "registry unavailable: " + e.Name + ": " + e.Message
}
