// Package queue provides pluggable message queue implementations for consuming registry events.
package queue

import (
	"context"
	"time"
)

// Event represents a registry lifecycle event
type Event struct {
	// ID is a unique identifier for the event
	ID string `json:"id"`

	// Type is the event type (e.g., "registered", "renewed", "revoked", "deprecated")
	Type string `json:"type"`

	// ANSName is the full ANSName identifier
	ANSName string `json:"ansName"`

	// FQDN is the stable fully qualified domain name
	FQDN string `json:"fqdn"`

	// Version is the agent version
	Version string `json:"version"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// RegistrarID identifies the originating registrar
	RegistrarID string `json:"registrarId"`

	// Signature is the JWS signature for verification
	Signature string `json:"signature"`

	// Meta contains additional event metadata
	Meta EventMeta `json:"meta"`

	// Raw contains the original event payload
	Raw []byte `json:"-"`
}

// EventMeta contains additional event metadata
type EventMeta struct {
	// Description is a human-readable agent description
	Description string `json:"description,omitempty"`

	// Capabilities is a list of agent capabilities
	Capabilities []string `json:"capabilities,omitempty"`

	// AgentCardURL is the URL to the full agent card
	AgentCardURL string `json:"agentCardUrl,omitempty"`

	// CapabilitiesHash is the hash of the agent card for integrity checks
	CapabilitiesHash string `json:"capabilitiesHash,omitempty"`

	// Protocols lists supported protocols
	Protocols []string `json:"protocols,omitempty"`

	// Endpoint is the agent's service endpoint
	Endpoint string `json:"endpoint,omitempty"`
}

// EventHandler is a function that processes events
type EventHandler func(ctx context.Context, event *Event) error

// Provider defines the interface for message queue implementations.
type Provider interface {
	// Subscribe starts consuming events and calls the handler for each event.
	// This is a blocking call that runs until the context is canceled.
	Subscribe(ctx context.Context, handler EventHandler) error

	// Publish sends an event to the queue (used for testing/federation).
	Publish(ctx context.Context, event *Event) error

	// Acknowledge confirms that an event has been processed.
	// For queues that don't support explicit acks, this is a no-op.
	Acknowledge(ctx context.Context, eventID string) error

	// Reject marks an event as failed for retry or dead-letter handling.
	Reject(ctx context.Context, eventID string, requeue bool) error

	// Stats returns queue statistics.
	Stats(ctx context.Context) (*Stats, error)

	// Close releases any resources held by the queue.
	Close() error

	// Name returns the provider name for logging/metrics.
	Name() string
}

// Stats contains queue statistics
type Stats struct {
	// Pending is the number of pending/unprocessed messages
	Pending int64

	// Processed is the total number of processed messages
	Processed int64

	// Failed is the number of failed messages
	Failed int64

	// ConsumerLag is the lag behind the latest message
	ConsumerLag int64

	// LastEventTime is when the last event was received
	LastEventTime time.Time
}

// Options contains common queue configuration options
type Options struct {
	// BufferSize is the internal buffer size for events
	BufferSize int

	// ConsumerGroup is the consumer group name
	ConsumerGroup string

	// ConsumerName is this consumer's unique name
	ConsumerName string

	// BlockTimeout is how long to block waiting for events
	BlockTimeout time.Duration

	// BatchSize is the maximum number of events to fetch at once
	BatchSize int

	// RetryAttempts is the number of retry attempts for failed events
	RetryAttempts int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration
}

// DefaultOptions returns sensible default queue options
func DefaultOptions() Options {
	return Options{
		BufferSize:    1000,
		ConsumerGroup: "ans-resolver",
		ConsumerName:  "resolver-1",
		BlockTimeout:  5 * time.Second,
		BatchSize:     100,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	}
}

// Factory is a function that creates a new queue provider
type Factory func(opts Options, config map[string]interface{}) (Provider, error)

// registry holds registered queue provider factories
var registry = make(map[string]Factory)

// Register registers a queue provider factory
func Register(name string, factory Factory) {
	registry[name] = factory
}

// New creates a new queue provider by name
func New(name string, opts Options, config map[string]interface{}) (Provider, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, &ErrUnknownProvider{Name: name}
	}
	return factory(opts, config)
}

// Available returns the names of all registered queue providers
func Available() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// ErrUnknownProvider is returned when an unknown queue provider is requested
type ErrUnknownProvider struct {
	Name string
}

func (e *ErrUnknownProvider) Error() string {
	return "unknown queue provider: " + e.Name
}
