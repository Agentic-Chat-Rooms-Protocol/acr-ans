// Package server provides the HTTP server implementation.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/route-ans/route-ans/internal/cache"
	"github.com/route-ans/route-ans/internal/config"
	"github.com/route-ans/route-ans/internal/queue"
	"github.com/route-ans/route-ans/internal/registry"
	"github.com/route-ans/route-ans/internal/resolver"
	"github.com/route-ans/route-ans/internal/telemetry"
	"github.com/route-ans/route-ans/internal/trust"
	"github.com/route-ans/route-ans/pkg/ansname"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/route-ans/route-ans/docs/swagger" // Swagger docs
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	version    string
	httpServer *http.Server
	metrics    *telemetry.Metrics

	// Core resolver (encapsulates cache, registry, trust)
	resolver *resolver.DefaultResolver

	// Keep references for health checks and direct access if needed
	cache    cache.Provider
	queue    queue.Provider
	registry registry.Adapter
}

// New creates a new server instance
func New(cfg *config.Config, version string) (*Server, error) {
	s := &Server{
		cfg:     cfg,
		version: version,
		metrics: telemetry.NewMetrics(
			cfg.Telemetry.Metrics.Namespace,
			cfg.Telemetry.Metrics.Subsystem,
		),
	}

	// Initialize providers
	if err := s.initProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:        s.routes(),
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	return s, nil
}

func (s *Server) initProviders() error {
	var err error

	// Initialize cache
	cacheOpts := cache.Options{
		DefaultTTL:      s.cfg.Cache.TTL.Default,
		MaxSize:         s.cfg.Cache.Memory.MaxSize,
		CleanupInterval: s.cfg.Cache.Memory.CleanupInterval,
		Namespace:       s.cfg.Cache.Redis.KeyPrefix,
	}

	// Build provider-specific config map
	cacheConfig := map[string]interface{}{
		"address":      s.cfg.Cache.Redis.Address,
		"password":     s.cfg.Cache.Redis.Password,
		"db":           s.cfg.Cache.Redis.DB,
		"poolSize":     s.cfg.Cache.Redis.PoolSize,
		"minIdleConns": s.cfg.Cache.Redis.MinIdleConns,
		"dialTimeout":  s.cfg.Cache.Redis.DialTimeout,
		"readTimeout":  s.cfg.Cache.Redis.ReadTimeout,
		"writeTimeout": s.cfg.Cache.Redis.WriteTimeout,
	}

	s.cache, err = cache.New(s.cfg.Cache.Provider, cacheOpts, cacheConfig)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	log.Info().Str("provider", s.cfg.Cache.Provider).Msg("Cache initialized")

	// Initialize queue
	if s.cfg.Queue.Enabled {
		queueOpts := queue.Options{
			BufferSize:    s.cfg.Queue.BufferSize,
			ConsumerGroup: s.cfg.Queue.RedisStreams.ConsumerGroup,
			ConsumerName:  s.cfg.Queue.RedisStreams.Consumer,
			BlockTimeout:  s.cfg.Queue.RedisStreams.BlockTimeout,
			BatchSize:     s.cfg.Queue.RedisStreams.BatchSize,
		}
		if queueOpts.BufferSize == 0 {
			queueOpts = queue.DefaultOptions()
		}

		queueConfig := map[string]interface{}{
			"address":  s.cfg.Queue.RedisStreams.Address,
			"password": s.cfg.Queue.RedisStreams.Password,
			"db":       s.cfg.Queue.RedisStreams.DB,
			"stream":   s.cfg.Queue.RedisStreams.Stream,
		}

		s.queue, err = queue.New(s.cfg.Queue.Provider, queueOpts, queueConfig)
		if err != nil {
			return fmt.Errorf("failed to create queue: %w", err)
		}
		log.Info().
			Str("provider", s.cfg.Queue.Provider).
			Int("bufferSize", s.cfg.Queue.BufferSize).
			Msg("Queue initialized")
	}

	// Initialize registry adapter (use first enabled one)
	for _, regCfg := range s.cfg.Registries {
		if !regCfg.Enabled {
			continue
		}
		regOpts := registry.Options{
			Timeout:      regCfg.Timeout,
			Retries:      regCfg.Retries,
			RetryBackoff: regCfg.RetryBackoff,
			Priority:     regCfg.Priority,
		}
		s.registry, err = registry.New(regCfg.Type, regOpts, regCfg.Config)
		if err != nil {
			log.Warn().Err(err).Str("registry", regCfg.Name).Msg("Failed to create registry adapter")
			continue
		}
		log.Info().Str("name", regCfg.Name).Str("type", regCfg.Type).Msg("Registry adapter initialized")
		break
	}

	if s.registry == nil {
		// Create a mock registry if none configured
		s.registry, _ = registry.New("mock", registry.DefaultOptions(), nil)
		log.Warn().Msg("No registry configured, using mock adapter")
	}

	// Initialize trust verifier
	var verifier trust.Verifier
	trustConfig := map[string]interface{}{
		"trustedRootsFile":      s.cfg.Trust.File.TrustedRootsFile,
		"trustedRegistrarsFile": s.cfg.Trust.File.TrustedRegistrarsFile,
	}
	trustProvider, err := trust.New(s.cfg.Trust.Provider, trust.Options{}, trustConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create trust provider, verification disabled")
	} else {
		verifierConfig := trust.VerifierConfig{
			RequireSignature:   s.cfg.Trust.Verification.RequireSignature,
			RequireMerkleProof: s.cfg.Trust.Verification.RequireMerkleProof,
			CheckRevocation:    s.cfg.Trust.Verification.CheckRevocation,
			OCSPEnabled:        s.cfg.Trust.Verification.OCSP.Enabled,
			OCSPTimeout:        s.cfg.Trust.Verification.OCSP.Timeout,
			CRLEnabled:         s.cfg.Trust.Verification.CRL.Enabled,
			CRLCacheTimeout:    s.cfg.Trust.Verification.CRL.CacheTimeout,
			GracePeriod:        s.cfg.Trust.Verification.AllowExpiredGracePeriod,
		}
		verifier = trust.NewVerifier(trustProvider, verifierConfig)
		log.Info().Str("provider", s.cfg.Trust.Provider).Msg("Trust provider initialized")
	}

	// Determine verification mode
	verificationMode := "disabled"
	if s.cfg.Trust.Verification.Enabled {
		verificationMode = "strict"
	}

	// Create the core resolver
	lookupTimeout := 10 * time.Second
	if len(s.cfg.Registries) > 0 {
		lookupTimeout = s.cfg.Registries[0].Timeout
	}
	resolverCfg := resolver.Config{
		DefaultTTL:       s.cfg.Cache.TTL.Default,
		VerifiedTTL:      s.cfg.Cache.TTL.Verified,
		RevokedTTL:       s.cfg.Cache.TTL.Revoked,
		NotFoundTTL:      s.cfg.Cache.TTL.NotFound,
		VerificationMode: verificationMode,
		ParallelLookups:  10,
		LookupTimeout:    lookupTimeout,
	}
	s.resolver = resolver.NewResolver(s.cache, s.registry, verifier, resolverCfg)
	log.Info().Msg("Resolver initialized")

	return nil
}

// routes sets up the HTTP routes
func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.loggingMiddleware)

	// Rate limiting
	if s.cfg.RateLimit.Enabled {
		limits := s.cfg.RateLimit.Limits
		if anon, ok := limits["anonymous"]; ok {
			r.Use(httprate.LimitByIP(anon.Requests, anon.Window))
		}
	}

	// Health endpoints
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// Metrics endpoint
	if s.cfg.Telemetry.Metrics.Enabled {
		r.Handle("/metrics", telemetry.Handler())
	}

	// Swagger UI endpoint
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// API v1
	r.Route("/v1", func(r chi.Router) {
		// Resolution endpoints
		r.Get("/resolve", s.handleResolve)
		r.Post("/resolve/batch", s.handleResolveBatch)

		// Stats
		r.Get("/stats", s.handleStats)
	})

	return r
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	log.Info().
		Str("addr", s.httpServer.Addr).
		Msg("Starting HTTP server")

	// Start metrics server if configured on different port
	if s.cfg.Telemetry.Metrics.Enabled &&
		s.cfg.Telemetry.Metrics.Port != s.cfg.Server.Port {
		go s.startMetricsServer()
	}

	// Start queue event processor if enabled
	if s.queue != nil {
		go s.processQueueEvents(ctx)
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) startMetricsServer() {
	mux := http.NewServeMux()
	mux.Handle(s.cfg.Telemetry.Metrics.Path, telemetry.Handler())

	addr := fmt.Sprintf("%s:%d",
		s.cfg.Telemetry.Metrics.Host,
		s.cfg.Telemetry.Metrics.Port,
	)

	log.Info().Str("addr", addr).Msg("Starting metrics server")

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Metrics server error")
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down server...")

	// Close providers
	if s.cache != nil {
		if err := s.cache.Close(); err != nil {
			log.Warn().Err(err).Msg("Error closing cache")
		}
	}
	if s.queue != nil {
		if err := s.queue.Close(); err != nil {
			log.Warn().Err(err).Msg("Error closing queue")
		}
	}
	if s.registry != nil {
		if err := s.registry.Close(); err != nil {
			log.Warn().Err(err).Msg("Error closing registry")
		}
	}

	return s.httpServer.Shutdown(ctx)
}

// processQueueEvents consumes and processes events from the queue
func (s *Server) processQueueEvents(ctx context.Context) {
	log.Info().Msg("Starting queue event processor")

	// Subscribe to queue events
	err := s.queue.Subscribe(ctx, func(ctx context.Context, event *queue.Event) error {
		start := time.Now()

		log.Debug().
			Str("eventID", event.ID).
			Str("type", event.Type).
			Str("ansName", event.ANSName).
			Time("timestamp", event.Timestamp).
			Msg("Processing queue event")

		// Process the event based on type
		if err := s.handleQueueEvent(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("eventID", event.ID).
				Str("type", event.Type).
				Str("ansName", event.ANSName).
				Msg("Failed to process queue event")

			// Reject the event for retry
			if rejectErr := s.queue.Reject(ctx, event.ID, true); rejectErr != nil {
				log.Warn().Err(rejectErr).Msg("Failed to reject event")
			}
			return err
		}

		// Acknowledge successful processing
		if ackErr := s.queue.Acknowledge(ctx, event.ID); ackErr != nil {
			log.Warn().Err(ackErr).Msg("Failed to acknowledge event")
		}

		log.Debug().
			Str("eventID", event.ID).
			Dur("duration", time.Since(start)).
			Msg("Queue event processed successfully")

		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Queue subscription ended with error")
	} else {
		log.Info().Msg("Queue event processor stopped")
	}
}

// handleQueueEvent processes a single queue event
func (s *Server) handleQueueEvent(ctx context.Context, event *queue.Event) error {
	switch event.Type {
	case "registered", "renewed":
		// Cache the new/updated registration
		log.Info().
			Str("ansName", event.ANSName).
			Str("type", event.Type).
			Msg("Agent registration event received")

		// If we have endpoint info, we could proactively cache it
		// For now, just invalidate to force fresh lookup
		if err := s.cache.Delete(ctx, event.ANSName); err != nil {
			log.Warn().Err(err).Msg("Failed to delete from cache")
		}

		// Update metrics
		s.metrics.RecordQueueEvent(event.Type, true)

	case "revoked", "deprecated":
		// Invalidate cache for revoked/deprecated agents
		log.Info().
			Str("ansName", event.ANSName).
			Str("type", event.Type).
			Msg("Agent revocation/deprecation event received")

		// Remove from cache
		if err := s.cache.Delete(ctx, event.ANSName); err != nil {
			log.Warn().Err(err).Msg("Failed to delete from cache")
		}

		// Update metrics
		s.metrics.RecordQueueEvent(event.Type, true)

	case "expired":
		// Handle expiration
		log.Info().
			Str("ansName", event.ANSName).
			Msg("Agent expiration event received")

		if err := s.cache.Delete(ctx, event.ANSName); err != nil {
			log.Warn().Err(err).Msg("Failed to delete from cache")
		}
		s.metrics.RecordQueueEvent(event.Type, true)

	default:
		log.Warn().
			Str("eventID", event.ID).
			Str("type", event.Type).
			Msg("Unknown event type, ignoring")
		s.metrics.RecordQueueEvent("unknown", false)
	}

	return nil
}

// Middleware
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("duration", duration).
			Int("bytes", ww.BytesWritten()).
			Msg("Request completed")

		// Record metrics
		s.metrics.RecordHTTPRequest(
			r.Method,
			r.URL.Path,
			ww.Status(),
			duration,
			int(r.ContentLength),
			ww.BytesWritten(),
		)
	})
}

// Handlers

// handleHealth godoc
// @Summary Health check
// @Description Returns server health status and version. Use for liveness probes.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"version": s.version,
	})
}

// handleReady godoc
// @Summary Readiness check
// @Description Checks if server can accept traffic by verifying registry connectivity. Use for readiness probes.
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /ready [get]
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check registry health
	healthy, err := s.registry.Healthy(r.Context())
	if err != nil || !healthy {
		s.jsonResponse(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "not ready",
			"error":  "registry unavailable",
		})
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}

// handleResolve godoc
// @Summary Resolve ANSName to endpoint
// @Description Resolves an ANSName to its verified endpoint with optional version negotiation. Performs cryptographic verification and caches results. Use this for agent discovery and connection establishment.
// @Tags Resolution
// @Produce json
// @Param name query string true "The full ANSName to resolve" example(mcp://agent.example.com)
// @Param version query string false "Version range for negotiation (e.g., '>=1.0.0', '^1.2.3', '~1.0.0', '1.x', '*')"
// @Param force query boolean false "Bypass cache and force fresh lookup"
// @Success 200 {object} ResolutionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /resolve [get]
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	// Get ANSName from query
	nameStr := r.URL.Query().Get("name")
	if nameStr == "" {
		s.errorResponse(w, http.StatusBadRequest, "missing 'name' query parameter")
		return
	}

	// Get optional version range for negotiation
	versionRange := r.URL.Query().Get("version")

	// Parse ANSName
	name, err := ansname.Parse(nameStr)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ANSName: %v", err))
		return
	}

	// Use the resolver to resolve the ANSName
	var record *resolver.ResolutionRecord
	if versionRange != "" {
		// Use version negotiation
		record, err = s.resolver.ResolveWithRange(ctx, name, versionRange)
	} else {
		// Standard exact version resolution
		record, err = s.resolver.Resolve(ctx, name)
	}
	if err != nil {
		var notFoundErr *resolver.ErrNotFound
		var verifyErr *resolver.ErrVerificationFailed
		if errors.As(err, &notFoundErr) {
			s.metrics.RecordResolution("not_found", name.Protocol, time.Since(start), false)
			s.errorResponse(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.As(err, &verifyErr) {
			s.metrics.RecordResolution("verification_failed", name.Protocol, time.Since(start), false)
			s.errorResponse(w, http.StatusUnprocessableEntity, fmt.Sprintf("verification failed: %s", verifyErr.Message))
			return
		}
		s.metrics.RecordResolution("error", name.Protocol, time.Since(start), false)
		s.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("resolution failed: %v", err))
		return
	}

	// Check status
	if record.Status == resolver.StatusRevoked {
		s.metrics.RecordResolution("revoked", name.Protocol, time.Since(start), false)
		s.errorResponse(w, http.StatusUnprocessableEntity, "certificate revoked")
		return
	}

	s.metrics.RecordResolution("success", name.Protocol, time.Since(start), false)

	s.jsonResponse(w, http.StatusOK, ResolutionResponse{
		Status:             record.Status,
		Agent:              record.ANSName,
		Protocol:           record.Protocol,
		Endpoint:           record.Endpoint,
		CertFingerprint:    record.CertFingerprint,
		ExpiresAt:          record.ExpiresAt.Format(time.RFC3339),
		ProtocolExtensions: record.ProtocolExtensions,
		Cached:             false, // The resolver handles caching internally
	})
}

// handleResolveBatch godoc
// @Summary Batch resolve multiple ANSNames
// @Description Resolves multiple ANSNames in parallel for efficiency. Returns results for all names, including errors. Use when discovering multiple agents simultaneously.
// @Tags Resolution
// @Accept json
// @Produce json
// @Param request body BatchResolveRequest true "Batch resolve request"
// @Success 200 {object} BatchResolveResponse
// @Failure 400 {object} ErrorResponse
// @Router /resolve/batch [post]
func (s *Server) handleResolveBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	results := make(map[string]*ResolutionResponse)

	for _, nameStr := range req.Names {
		name, err := ansname.Parse(nameStr)
		if err != nil {
			results[nameStr] = &ResolutionResponse{
				Status: "error",
				Error:  fmt.Sprintf("invalid ANSName: %v", err),
			}
			continue
		}

		record, err := s.resolver.Resolve(ctx, name)
		if err != nil {
			status := "error"
			var notFoundErr *resolver.ErrNotFound
			var verifyErr *resolver.ErrVerificationFailed
			if errors.As(err, &notFoundErr) {
				status = "not_found"
			} else if errors.As(err, &verifyErr) {
				status = "verification_failed"
			}
			results[nameStr] = &ResolutionResponse{
				Status: status,
				Error:  err.Error(),
			}
			continue
		}

		results[nameStr] = &ResolutionResponse{
			Status:          "verified",
			Agent:           record.ANSName,
			Protocol:        record.Protocol,
			Endpoint:        record.Endpoint,
			CertFingerprint: record.CertFingerprint,
			ExpiresAt:       record.ExpiresAt.Format(time.RFC3339),
		}
	}

	s.jsonResponse(w, http.StatusOK, BatchResolveResponse{Results: results})
}

// handleStats godoc
// @Summary Get resolver statistics
// @Description Returns operational metrics including cache hit rates, queue depth, and resolution counts. Use for monitoring and observability.
// @Tags Stats
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} ErrorResponse
// @Router /stats [get]
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.resolver.Stats(r.Context())
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"resolver": stats,
		"version":  s.version,
	})
}

// Response helpers

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

// Response types

// ResolutionResponse contains the result of an ANS resolution request
type ResolutionResponse struct {
	Status             string                 `json:"status"`
	Agent              string                 `json:"agent,omitempty"`
	Protocol           string                 `json:"protocol,omitempty"`
	Endpoint           string                 `json:"endpoint,omitempty"`
	CertFingerprint    string                 `json:"certFingerprint,omitempty"`
	ExpiresAt          string                 `json:"expiresAt,omitempty"`
	ProtocolExtensions map[string]interface{} `json:"protocolExtensions,omitempty"`
	Cached             bool                   `json:"cached,omitempty"`
	Error              string                 `json:"error,omitempty"`
}

// BatchResolveRequest contains a list of ANS names to resolve
type BatchResolveRequest struct {
	Names []string `json:"names"`
}

// BatchResolveResponse contains results for multiple ANS name resolutions
type BatchResolveResponse struct {
	Results map[string]*ResolutionResponse `json:"results"`
}

// VerifyResponse contains verification results for an ANS name
type VerifyResponse struct {
	Valid      bool                             `json:"valid"`
	VerifiedAt string                           `json:"verifiedAt"`
	Checks     map[string]*resolver.CheckResult `json:"checks"`
}

// ErrorResponse contains error details for failed requests
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
