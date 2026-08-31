// Package models contains shared data types used across the resolution server.
package models

import (
	"time"
)

// ResolutionRecord represents a verified agent resolution record with cryptographic proof
type ResolutionRecord struct {
	// ANSName is the full ANSName identifier
	ANSName string `json:"ansName"`

	// FQDN is the stable fully qualified domain name
	FQDN string `json:"fqdn"`

	// Protocol is the agent's primary protocol
	Protocol string `json:"protocol"`

	// Version is the agent version
	Version string `json:"version"`

	// Status is the verification/agent status
	Status string `json:"status"`

	// Endpoint is the agent's service endpoint URL
	Endpoint string `json:"endpoint"`

	// CertFingerprint is the SHA-256 fingerprint of the public certificate
	CertFingerprint string `json:"certFingerprint"`

	// CertExpiresAt is when the public certificate expires
	CertExpiresAt time.Time `json:"certExpiresAt"`

	// DisplayName is a human-readable name
	DisplayName string `json:"displayName,omitempty"`

	// Description describes the agent's purpose
	Description string `json:"description,omitempty"`

	// Capabilities lists the agent's capabilities
	Capabilities []string `json:"capabilities,omitempty"`

	// Protocols lists all supported protocols
	Protocols []string `json:"protocols,omitempty"`

	// ProtocolExtensions contains protocol-specific metadata
	ProtocolExtensions map[string]interface{} `json:"protocolExtensions,omitempty"`

	// RegistrarID identifies the issuing registrar
	RegistrarID string `json:"registrarId"`

	// TTL is the cache time-to-live
	TTL time.Duration `json:"ttl"`

	// RegisteredAt is when the agent was first registered
	RegisteredAt time.Time `json:"registeredAt"`

	// UpdatedAt is when the record was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// ExpiresAt is when the record/certificate expires
	ExpiresAt time.Time `json:"expiresAt"`

	// VerifiedAt is when the record was last verified
	VerifiedAt time.Time `json:"verifiedAt"`

	// Verification contains verification proof data
	Verification *VerificationProof `json:"verification,omitempty"`
}

// VerificationProof contains cryptographic proof of verification
type VerificationProof struct {
	// MerkleProof contains the Merkle inclusion proof
	MerkleProof *MerkleProofData `json:"merkleProof,omitempty"`

	// SignatureValid indicates if the registry signature was valid
	SignatureValid bool `json:"signatureValid"`

	// CertificateValid indicates if the certificate chain was valid
	CertificateValid bool `json:"certificateValid"`

	// RevocationChecked indicates if revocation was checked
	RevocationChecked bool `json:"revocationChecked"`

	// NotRevoked indicates the certificate is not revoked
	NotRevoked bool `json:"notRevoked"`
}

// MerkleProofData contains Merkle proof details
type MerkleProofData struct {
	// TreeSize is the size of the tree
	TreeSize int64 `json:"treeSize"`

	// LeafIndex is the leaf index in the tree
	LeafIndex int64 `json:"leafIndex"`

	// Hashes contains the proof hashes (hex-encoded)
	Hashes []string `json:"hashes"`

	// RootHash is the expected root hash (hex-encoded)
	RootHash string `json:"rootHash"`
}

// Status constants for ResolutionRecord
const (
	// StatusActive indicates an active, verified agent
	StatusActive     = "active"
	StatusPending    = "pending"
	StatusRevoked    = "revoked"
	StatusExpired    = "expired"
	StatusDeprecated = "deprecated"
	StatusInvalid    = "invalid"
	StatusUnknown    = "unknown"
)

// SearchQuery represents a search request
type SearchQuery struct {
	// Text is a free-text search query
	Text string

	// Protocol filters by protocol (e.g., "a2a", "mcp")
	Protocol string

	// ProviderID filters by provider ID
	ProviderID string

	// Capability filters by capability
	Capability string

	// Status filters by status (e.g., "active", "revoked")
	Status string

	// Capabilities filters by capabilities (agent must have all listed)
	Capabilities []string

	// Tags filters by tags
	Tags []string

	// Limit is the maximum number of results
	Limit int

	// Offset is the pagination offset
	Offset int

	// OrderBy specifies the sort field
	OrderBy string

	// OrderDesc specifies descending order
	OrderDesc bool
}

// SearchResult contains search results with pagination info
type SearchResult struct {
	// Records is the list of matching records
	Records []*ResolutionRecord

	// Total is the total number of matching records (before pagination)
	Total int64

	// Limit is the limit used in the query
	Limit int

	// Offset is the offset used in the query
	Offset int

	// HasMore indicates if there are more results
	HasMore bool
}

// VerifyResult contains the result of a verification request
type VerifyResult struct {
	// Valid indicates if verification passed
	Valid bool `json:"valid"`

	// Status is the verification status
	Status string `json:"status"`

	// Record is the resolved record (if found)
	Record *ResolutionRecord `json:"record,omitempty"`

	// Checks contains individual check results
	Checks map[string]*CheckResult `json:"checks,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`

	// VerifiedAt is when verification was performed
	VerifiedAt time.Time `json:"verifiedAt"`
}

// CheckResult contains the result of a single verification check
type CheckResult struct {
	// Name is the check name
	Name string `json:"name"`

	// Passed indicates if the check passed
	Passed bool `json:"passed"`

	// Required indicates if this check is required for overall success
	Required bool `json:"required"`

	// Message contains a human-readable message
	Message string `json:"message,omitempty"`

	// Details contains additional check-specific details
	Details map[string]interface{} `json:"details,omitempty"`

	// Duration is how long the check took (in nanoseconds)
	Duration time.Duration `json:"duration,omitempty" swaggertype:"integer"`
}

// Stats contains resolver statistics
type Stats struct {
	TotalRequests        int64   `json:"totalRequests"`
	CacheHits            int64   `json:"cacheHits"`
	CacheMisses          int64   `json:"cacheMisses"`
	RegistryLookups      int64   `json:"registryLookups"`
	VerificationSuccess  int64   `json:"verificationSuccess"`
	VerificationFailures int64   `json:"verificationFailures"`
	AverageLatencyMs     float64 `json:"averageLatencyMs"`
}

// Verification status constants
const (
	VerificationPassed  = "passed"
	VerificationFailed  = "failed"
	VerificationError   = "error"
	VerificationSkipped = "skipped"
)
