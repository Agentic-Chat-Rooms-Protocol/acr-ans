// Package main is the entry point for the ANS Resolution Server.
// It initializes all components and starts the HTTP server.
//
// @title Route ANS API
// @version 1.0.0
// @description High-performance, cryptographically-verified resolution of ANSName identifiers to agent endpoints.
// @termsOfService https://github.com/route-ans/route-ans
//
// @contact.name Route ANS Team
// @contact.url https://github.com/route-ans/route-ans
//
// @license.name Apache 2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0
//
// @host localhost:8080
// @BasePath /v1
// @schemes http https
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/route-ans/route-ans/internal/config"
	"github.com/route-ans/route-ans/internal/server"
	"github.com/route-ans/route-ans/internal/telemetry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Build-time variables set via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "configs/resolver.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Show version and exit if requested
	if *showVersion {
		fmt.Printf("ANS Resolution Server\n")
		fmt.Printf("  Version:    %s\n", version)
		fmt.Printf("  Build Time: %s\n", buildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	logger := telemetry.NewLogger(cfg.Telemetry.Logging)
	log.Logger = logger

	log.Info().
		Str("version", version).
		Str("buildTime", buildTime).
		Str("configPath", *configPath).
		Msg("Starting ANS Resolution Server")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize telemetry
	if cfg.Telemetry.Tracing.Enabled {
		shutdown, err := telemetry.InitTracing(ctx, cfg.Telemetry.Tracing)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize tracing, continuing without it")
		} else {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				if err := shutdown(shutdownCtx); err != nil {
					log.Error().Err(err).Msg("Error shutting down tracer")
				}
			}()
		}
	}

	// Create and start the server
	srv, err := server.New(cfg, version)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create server")
	}

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Error().Err(err).Msg("Server error")
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	}

	// Graceful shutdown
	log.Info().Msg("Initiating graceful shutdown...")
	shutdownTimeout := cfg.Server.GracefulShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Server shutdown complete")
}

func init() {
	// Set up zerolog defaults
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.DurationFieldUnit = time.Millisecond
}
