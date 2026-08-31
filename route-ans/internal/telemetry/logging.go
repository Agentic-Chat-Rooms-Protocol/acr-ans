// Package telemetry provides logging, metrics, and tracing utilities.
package telemetry

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/route-ans/route-ans/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// NewLogger creates a new zerolog logger based on configuration
func NewLogger(cfg config.LoggingConfig) zerolog.Logger {
	var output io.Writer

	switch cfg.Output {
	case "stderr":
		output = os.Stderr
	case "file":
		// For file output, you'd typically use lumberjack for rotation
		// For now, just use stdout
		output = os.Stdout
	default:
		output = os.Stdout
	}

	// Configure format
	switch cfg.Format {
	case "text", "pretty":
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	default:
		// JSON format (default)
	}

	// Configure level
	var level zerolog.Level
	switch cfg.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "warn", "warning":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	default:
		level = zerolog.InfoLevel
	}

	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	return logger
}

// InitTracing initializes OpenTelemetry tracing
func InitTracing(ctx context.Context, cfg config.TracingConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(ctx context.Context) error { return nil }, nil
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create OTLP exporter
	var exporter sdktrace.SpanExporter

	switch cfg.Provider {
	case "otlp":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLP.Endpoint),
		}
		if cfg.OTLP.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, err
		}
	default:
		// Default to OTLP
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLP.Endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info().
		Str("provider", cfg.Provider).
		Str("endpoint", cfg.OTLP.Endpoint).
		Float64("sampleRate", cfg.SampleRate).
		Msg("Tracing initialized")

	return tp.Shutdown, nil
}
