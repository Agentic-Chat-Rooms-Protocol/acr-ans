// Package resolver contains the core resolution logic.
package resolver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/route-ans/route-ans/internal/cache"
	"github.com/route-ans/route-ans/internal/registry"
	"github.com/route-ans/route-ans/internal/trust"
	"github.com/route-ans/route-ans/pkg/ansname"
	"github.com/rs/zerolog/log"
)

// DefaultResolver implements the Resolver interface
type DefaultResolver struct {
	cache    cache.Provider
	registry registry.Adapter
	verifier trust.Verifier

	// Configuration
	defaultTTL       time.Duration
	verifiedTTL      time.Duration
	revokedTTL       time.Duration
	notFoundTTL      time.Duration
	verificationMode string // "strict", "permissive", "disabled"
	parallelLookups  int
	lookupTimeout    time.Duration

	// Stats
	stats struct {
		TotalRequests        int64
		CacheHits            int64
		CacheMisses          int64
		RegistryLookups      int64
		VerificationSuccess  int64
		VerificationFailures int64
		TotalLatencyNs       int64
		RequestCount         int64
	}
	statsLock sync.RWMutex
}

// Config contains resolver configuration
type Config struct {
	DefaultTTL       time.Duration
	VerifiedTTL      time.Duration
	RevokedTTL       time.Duration
	NotFoundTTL      time.Duration
	VerificationMode string
	ParallelLookups  int
	LookupTimeout    time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		DefaultTTL:       5 * time.Minute,
		VerifiedTTL:      10 * time.Minute,
		RevokedTTL:       1 * time.Hour,
		NotFoundTTL:      1 * time.Minute,
		VerificationMode: "strict",
		ParallelLookups:  10,
		LookupTimeout:    10 * time.Second,
	}
}

// NewResolver creates a new resolver instance
func NewResolver(
	cacheProvider cache.Provider,
	registryAdapter registry.Adapter,
	verifier trust.Verifier,
	cfg Config,
) *DefaultResolver {
	return &DefaultResolver{
		cache:            cacheProvider,
		registry:         registryAdapter,
		verifier:         verifier,
		defaultTTL:       cfg.DefaultTTL,
		verifiedTTL:      cfg.VerifiedTTL,
		revokedTTL:       cfg.RevokedTTL,
		notFoundTTL:      cfg.NotFoundTTL,
		verificationMode: cfg.VerificationMode,
		parallelLookups:  cfg.ParallelLookups,
		lookupTimeout:    cfg.LookupTimeout,
	}
}

// Resolve resolves an ANSName to a verified endpoint
//
//nolint:gocyclo // High complexity due to multi-step resolution with caching, lookup, and verification
func (r *DefaultResolver) Resolve(ctx context.Context, name *ansname.ANSName) (*ResolutionRecord, error) {
	start := time.Now()
	atomic.AddInt64(&r.stats.TotalRequests, 1)

	defer func() {
		atomic.AddInt64(&r.stats.TotalLatencyNs, time.Since(start).Nanoseconds())
		atomic.AddInt64(&r.stats.RequestCount, 1)
	}()

	// Step 1: Check cache
	cacheKey := name.CacheKey()
	cachedVal, err := r.cache.Get(ctx, cacheKey)
	if err != nil {
		log.Warn().Err(err).Str("key", cacheKey).Msg("Cache get error")
	}

	if cachedVal != nil {
		if cached, ok := cachedVal.(*ResolutionRecord); ok {
			atomic.AddInt64(&r.stats.CacheHits, 1)
			log.Debug().Str("ansName", name.String()).Msg("Cache hit")
			return cached, nil
		}
	}

	atomic.AddInt64(&r.stats.CacheMisses, 1)
	log.Debug().Str("ansName", name.String()).Msg("Cache miss, looking up registry")

	// Step 2: Lookup from registry
	atomic.AddInt64(&r.stats.RegistryLookups, 1)

	lookupCtx, cancel := context.WithTimeout(ctx, r.lookupTimeout)
	defer cancel()

	record, err := r.registry.Lookup(lookupCtx, name)
	if err != nil {
		var notFoundErr *registry.ErrNotFound
		if errors.As(err, &notFoundErr) {
			// Cache the not-found result to prevent repeated lookups
			log.Debug().Str("ansName", name.String()).Msg("Agent not found in registry")
			return nil, &ErrNotFound{ANSName: name.String()}
		}
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}

	// Step 3: Verify the record (if verification is enabled)
	resRecord := record.ToResolutionRecord()

	if r.verificationMode != "disabled" && r.verifier != nil {
		verifyResult, err := r.verifier.VerifyRegistryRecord(ctx, record)
		if err != nil {
			log.Error().Err(err).Str("ansName", name.String()).Msg("Verification error")
			if r.verificationMode == "strict" {
				atomic.AddInt64(&r.stats.VerificationFailures, 1)
				return nil, fmt.Errorf("verification failed: %w", err)
			}
			// In permissive mode, log but continue
		} else if !verifyResult.Valid {
			atomic.AddInt64(&r.stats.VerificationFailures, 1)
			if r.verificationMode == "strict" {
				return nil, &ErrVerificationFailed{
					ANSName: name.String(),
					Status:  string(verifyResult.Status),
					Message: verifyResult.Error,
				}
			}
			// In permissive mode, mark as unverified but continue
			resRecord.Status = StatusInvalid
		} else {
			atomic.AddInt64(&r.stats.VerificationSuccess, 1)
			resRecord.Status = StatusActive
			resRecord.Verification = &VerificationProof{
				SignatureValid:    true,
				CertificateValid:  true,
				RevocationChecked: verifyResult.Checks[trust.CheckRevocation] != nil,
				NotRevoked:        verifyResult.Checks[trust.CheckRevocation] != nil && verifyResult.Checks[trust.CheckRevocation].Passed,
			}
		}
	}

	resRecord.VerifiedAt = time.Now()

	// Step 4: Determine TTL based on status
	var ttl time.Duration
	switch resRecord.Status {
	case StatusActive:
		ttl = r.verifiedTTL
	case StatusRevoked:
		ttl = r.revokedTTL
	default:
		ttl = r.defaultTTL
	}

	// Step 5: Cache the result
	if err := r.cache.Set(ctx, cacheKey, resRecord, ttl); err != nil {
		log.Warn().Err(err).Str("key", cacheKey).Msg("Cache set error")
	}

	return resRecord, nil
}

// ResolveRaw resolves an ANSName string to a verified endpoint
func (r *DefaultResolver) ResolveRaw(ctx context.Context, nameStr string) (*ResolutionRecord, error) {
	name, err := ansname.Parse(nameStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ANSName: %w", err)
	}
	return r.Resolve(ctx, name)
}

// ResolveWithRange resolves an ANSName with version range negotiation.
// This implements version negotiation.
// If multiple versions match the range, returns the highest compatible version.
func (r *DefaultResolver) ResolveWithRange(ctx context.Context, name *ansname.ANSName, versionRange string) (*ResolutionRecord, error) {
	start := time.Now()
	atomic.AddInt64(&r.stats.TotalRequests, 1)

	defer func() {
		atomic.AddInt64(&r.stats.TotalLatencyNs, time.Since(start).Nanoseconds())
		atomic.AddInt64(&r.stats.RequestCount, 1)
	}()

	// Step 1: Parse version range
	if versionRange == "" {
		versionRange = "*" // Default to any version
	}

	// Step 2: Lookup all versions by FQDN
	fqdn := name.FQDN()
	log.Debug().Str("fqdn", fqdn).Str("range", versionRange).Msg("Looking up versions for negotiation")

	lookupCtx, cancel := context.WithTimeout(ctx, r.lookupTimeout)
	defer cancel()

	atomic.AddInt64(&r.stats.RegistryLookups, 1)
	records, err := r.registry.LookupByFQDN(lookupCtx, fqdn)
	if err != nil {
		var notFoundErr *registry.ErrNotFound
		if errors.As(err, &notFoundErr) {
			log.Debug().Str("fqdn", fqdn).Msg("Agent not found in registry")
			return nil, &ErrNotFound{ANSName: fqdn}
		}
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}

	if len(records) == 0 {
		return nil, &ErrNotFound{ANSName: fqdn}
	}

	// Step 3: Convert registry records to ANSNames for version negotiation
	candidates := make([]*ansname.ANSName, 0, len(records))
	for _, record := range records {
		candidate, err := ansname.Parse(record.ANSName)
		if err != nil {
			log.Warn().Err(err).Str("ansName", record.ANSName).Msg("Failed to parse candidate ANSName")
			continue
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no valid candidate versions found")
	}

	// Step 4: Perform version negotiation
	selectedVersion, err := ansname.NegotiateVersion(candidates, versionRange)
	if err != nil {
		return nil, fmt.Errorf("version negotiation failed: %w", err)
	}

	log.Info().
		Str("fqdn", fqdn).
		Str("range", versionRange).
		Str("selected", selectedVersion.Version).
		Int("candidates", len(candidates)).
		Msg("Version negotiation successful")

	// Step 5: Resolve the selected version (this will handle caching and verification)
	return r.Resolve(ctx, selectedVersion)
}

// ResolveBatch resolves multiple ANSNames in parallel
func (r *DefaultResolver) ResolveBatch(ctx context.Context, names []*ansname.ANSName) ([]*ResolutionRecord, error) {
	results := make([]*ResolutionRecord, len(names))
	errors := make([]error, len(names))
	var wg sync.WaitGroup

	// Use semaphore to limit parallelism
	sem := make(chan struct{}, r.parallelLookups)

	for i, name := range names {
		wg.Add(1)
		go func(idx int, n *ansname.ANSName) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			record, err := r.Resolve(ctx, n)
			results[idx] = record
			errors[idx] = err
		}(i, name)
	}

	wg.Wait()

	// Check for any errors
	var firstErr error
	for _, err := range errors {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return results, firstErr
}

// Verify performs explicit verification of an ANSName
func (r *DefaultResolver) Verify(ctx context.Context, name *ansname.ANSName) (*VerifyResult, error) {
	// First resolve the record
	record, err := r.registry.Lookup(ctx, name)
	if err != nil {
		return &VerifyResult{
			Valid:      false,
			Status:     VerificationError,
			Error:      err.Error(),
			VerifiedAt: time.Now(),
		}, err
	}

	// Perform verification
	if r.verifier == nil {
		return &VerifyResult{
			Valid:      true,
			Status:     "unverified",
			Record:     record.ToResolutionRecord(),
			VerifiedAt: time.Now(),
			Checks:     make(map[string]*CheckResult),
		}, nil
	}

	verifyResult, err := r.verifier.VerifyRegistryRecord(ctx, record)
	if err != nil {
		return &VerifyResult{
			Valid:      false,
			Status:     VerificationError,
			Error:      err.Error(),
			VerifiedAt: time.Now(),
		}, err
	}

	// Convert trust.CheckResult to resolver.CheckResult
	checks := make(map[string]*CheckResult)
	for k, v := range verifyResult.Checks {
		checks[k] = &CheckResult{
			Passed:   v.Passed,
			Required: v.Required,
			Message:  v.Message,
			Details:  v.Details,
		}
	}

	resRecord := record.ToResolutionRecord()
	resRecord.VerifiedAt = time.Now()

	return &VerifyResult{
		Valid:      verifyResult.Valid,
		Status:     string(verifyResult.Status),
		Record:     resRecord,
		Checks:     checks,
		VerifiedAt: time.Now(),
	}, nil
}

// Invalidate invalidates the cache for an ANSName
func (r *DefaultResolver) Invalidate(ctx context.Context, name *ansname.ANSName) error {
	cacheKey := name.CacheKey()
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("cache invalidation failed: %w", err)
	}
	log.Debug().Str("key", cacheKey).Msg("Cache invalidated")
	return nil
}

// Stats returns resolver statistics
func (r *DefaultResolver) Stats(ctx context.Context) (*Stats, error) {
	r.statsLock.RLock()
	defer r.statsLock.RUnlock()

	totalRequests := atomic.LoadInt64(&r.stats.TotalRequests)
	cacheHits := atomic.LoadInt64(&r.stats.CacheHits)
	cacheMisses := atomic.LoadInt64(&r.stats.CacheMisses)
	registryLookups := atomic.LoadInt64(&r.stats.RegistryLookups)
	verificationSuccess := atomic.LoadInt64(&r.stats.VerificationSuccess)
	verificationFailures := atomic.LoadInt64(&r.stats.VerificationFailures)
	totalLatencyNs := atomic.LoadInt64(&r.stats.TotalLatencyNs)
	requestCount := atomic.LoadInt64(&r.stats.RequestCount)

	var avgLatencyMs float64
	if requestCount > 0 {
		avgLatencyMs = float64(totalLatencyNs) / float64(requestCount) / 1e6
	}

	return &Stats{
		TotalRequests:        totalRequests,
		CacheHits:            cacheHits,
		CacheMisses:          cacheMisses,
		RegistryLookups:      registryLookups,
		VerificationSuccess:  verificationSuccess,
		VerificationFailures: verificationFailures,
		AverageLatencyMs:     avgLatencyMs,
	}, nil
}

// ProcessEvent handles registry events for cache invalidation
func (r *DefaultResolver) ProcessEvent(ctx context.Context, event *registry.Event) error {
	switch event.Type {
	case "revoked", "deprecated", "updated":
		// Parse the ANSName and invalidate cache
		name, err := ansname.Parse(event.ANSName)
		if err != nil {
			log.Warn().Err(err).Str("ansName", event.ANSName).Msg("Failed to parse event ANSName")
			return nil
		}
		return r.Invalidate(ctx, name)
	case "registered":
		// New registration, nothing to invalidate
		log.Debug().Str("ansName", event.ANSName).Msg("New registration event")
		return nil
	default:
		log.Debug().Str("type", event.Type).Msg("Unknown event type")
		return nil
	}
}

// Error types

// ErrNotFound is returned when an agent is not found
type ErrNotFound struct {
	ANSName string
}

func (e *ErrNotFound) Error() string {
	return "agent not found: " + e.ANSName
}

// ErrVerificationFailed is returned when verification fails
type ErrVerificationFailed struct {
	ANSName string
	Status  string
	Message string
}

func (e *ErrVerificationFailed) Error() string {
	return fmt.Sprintf("verification failed for %s: %s - %s", e.ANSName, e.Status, e.Message)
}
