// Package queue provides pluggable message queue implementations.
package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// memoryQueue is an in-memory queue implementation for testing/development
type memoryQueue struct {
	mu       sync.RWMutex
	buffer   chan *Event
	handlers []EventHandler
	closed   atomic.Bool
	wg       sync.WaitGroup

	// Stats
	processed atomic.Int64
	failed    atomic.Int64
	lastEvent atomic.Value // stores time.Time
}

// NewMemoryQueue creates a new in-memory queue
func NewMemoryQueue(opts Options, config map[string]interface{}) (Provider, error) {
	if opts.BufferSize <= 0 {
		opts.BufferSize = 1000
	}

	q := &memoryQueue{
		buffer:   make(chan *Event, opts.BufferSize),
		handlers: make([]EventHandler, 0),
	}
	q.lastEvent.Store(time.Time{})

	return q, nil
}

// Subscribe starts consuming events
func (q *memoryQueue) Subscribe(ctx context.Context, handler EventHandler) error {
	q.mu.Lock()
	q.handlers = append(q.handlers, handler)
	q.mu.Unlock()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-q.buffer:
				if !ok {
					return
				}
				if err := handler(ctx, event); err != nil {
					q.failed.Add(1)
				} else {
					q.processed.Add(1)
				}
				q.lastEvent.Store(time.Now())
			}
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

// Publish sends an event to the queue
func (q *memoryQueue) Publish(ctx context.Context, event *Event) error {
	if q.closed.Load() {
		return ErrQueueClosed
	}

	select {
	case q.buffer <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrQueueFull
	}
}

// Acknowledge confirms event processing (no-op for memory queue)
func (q *memoryQueue) Acknowledge(ctx context.Context, eventID string) error {
	return nil
}

// Reject marks an event as failed (no-op for memory queue)
// For memory queue, we can't requeue. In a real implementation, this would add to a retry queue.
func (q *memoryQueue) Reject(ctx context.Context, eventID string, requeue bool) error {
	_ = requeue // Ignored: memory queue doesn't support requeueing
	return nil
}

// Stats returns queue statistics
func (q *memoryQueue) Stats(ctx context.Context) (*Stats, error) {
	lastEvent := q.lastEvent.Load().(time.Time)

	return &Stats{
		Pending:       int64(len(q.buffer)),
		Processed:     q.processed.Load(),
		Failed:        q.failed.Load(),
		ConsumerLag:   0,
		LastEventTime: lastEvent,
	}, nil
}

// Close releases resources
func (q *memoryQueue) Close() error {
	if q.closed.CompareAndSwap(false, true) {
		close(q.buffer)
		q.wg.Wait()
	}
	return nil
}

const queueProviderName = "memory"

// Name returns the provider name
func (q *memoryQueue) Name() string {
	return queueProviderName
}

// Common errors
var (
	ErrQueueClosed = &Error{Message: "queue is closed"}
	ErrQueueFull   = &Error{Message: "queue is full"}
)

// Error represents a queue error
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func init() {
	Register(queueProviderName, func(opts Options, config map[string]interface{}) (Provider, error) {
		return NewMemoryQueue(opts, config)
	})
}
