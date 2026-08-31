// Package resolver contains the core resolution logic.
package resolver

import (
	"context"

	"github.com/route-ans/route-ans/internal/models"
	"github.com/route-ans/route-ans/pkg/ansname"
)

// Re-export types from models for convenience
type (
	ResolutionRecord  = models.ResolutionRecord
	VerificationProof = models.VerificationProof
	MerkleProofData   = models.MerkleProofData
	SearchQuery       = models.SearchQuery
	SearchResult      = models.SearchResult
	VerifyResult      = models.VerifyResult
	CheckResult       = models.CheckResult
	Stats             = models.Stats
)

// Re-export status constants
const (
	StatusActive     = models.StatusActive
	StatusPending    = models.StatusPending
	StatusRevoked    = models.StatusRevoked
	StatusExpired    = models.StatusExpired
	StatusDeprecated = models.StatusDeprecated
	StatusInvalid    = models.StatusInvalid
	StatusUnknown    = models.StatusUnknown
)

// Re-export verification constants
const (
	VerificationPassed  = models.VerificationPassed
	VerificationFailed  = models.VerificationFailed
	VerificationError   = models.VerificationError
	VerificationSkipped = models.VerificationSkipped
)

// Resolver defines the interface for the core resolution service
type Resolver interface {
	// Resolve resolves an ANSName to a verified endpoint
	Resolve(ctx context.Context, name *ansname.ANSName) (*ResolutionRecord, error)

	// ResolveRaw resolves an ANSName string to a verified endpoint
	ResolveRaw(ctx context.Context, nameStr string) (*ResolutionRecord, error)

	// ResolveWithRange resolves an ANSName with version range negotiation.
	// The version field in the ANSName can be a version range (e.g., ">=1.0.0", "^1.2.3", "~1.0.0")
	// Returns the best matching version based on the range.
	ResolveWithRange(ctx context.Context, name *ansname.ANSName, versionRange string) (*ResolutionRecord, error)

	// ResolveBatch resolves multiple ANSNames in parallel
	ResolveBatch(ctx context.Context, names []*ansname.ANSName) ([]*ResolutionRecord, error)

	// Verify performs explicit verification of an ANSName
	Verify(ctx context.Context, name *ansname.ANSName) (*VerifyResult, error)

	// Invalidate invalidates the cache for an ANSName
	Invalidate(ctx context.Context, name *ansname.ANSName) error

	// Stats returns resolver statistics
	Stats(ctx context.Context) (*Stats, error)
}
