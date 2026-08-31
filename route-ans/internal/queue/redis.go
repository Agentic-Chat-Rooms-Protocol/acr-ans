// Package queue provides Redis Streams queue implementation.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// redisQueue implements the Provider interface using Redis Streams
type redisQueue struct {
	client        *redis.Client
	stream        string
	consumerGroup string
	consumerName  string
	blockTimeout  time.Duration
	batchSize     int64

	closed atomic.Bool
	wg     sync.WaitGroup

	// Stats
	processed atomic.Int64
	failed    atomic.Int64
	lastEvent atomic.Value // stores time.Time
}

// RedisQueueOptions contains Redis Streams configuration
type RedisQueueOptions struct {
	Address       string
	Password      string
	DB            int
	Stream        string
	ConsumerGroup string
	ConsumerName  string
	BlockTimeout  time.Duration
	BatchSize     int64
}

// DefaultRedisQueueOptions returns sensible defaults
func DefaultRedisQueueOptions() RedisQueueOptions {
	return RedisQueueOptions{
		Address:       "localhost:6379",
		Password:      "",
		DB:            0,
		Stream:        "ans:events",
		ConsumerGroup: "resolver-group",
		ConsumerName:  "resolver-1",
		BlockTimeout:  5 * time.Second,
		BatchSize:     10,
	}
}

// NewRedisQueue creates a new Redis Streams queue provider
func NewRedisQueue(opts RedisQueueOptions) (Provider, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     opts.Address,
		Password: opts.Password,
		DB:       opts.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create consumer group if it doesn't exist
	err := client.XGroupCreateMkStream(ctx, opts.Stream, opts.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		// Ignore error if group already exists
		log.Debug().Err(err).Msg("Consumer group creation (may already exist)")
	}

	q := &redisQueue{
		client:        client,
		stream:        opts.Stream,
		consumerGroup: opts.ConsumerGroup,
		consumerName:  opts.ConsumerName,
		blockTimeout:  opts.BlockTimeout,
		batchSize:     opts.BatchSize,
	}
	q.lastEvent.Store(time.Time{})

	return q, nil
}

// Subscribe starts consuming events from Redis Streams
func (q *redisQueue) Subscribe(ctx context.Context, handler EventHandler) error {
	q.wg.Add(1)
	defer q.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if q.closed.Load() {
			return nil
		}

		// Read from stream
		streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.consumerGroup,
			Consumer: q.consumerName,
			Streams:  []string{q.stream, ">"},
			Count:    q.batchSize,
			Block:    q.blockTimeout,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue // No messages, try again
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Error().Err(err).Msg("Redis XReadGroup error")
			time.Sleep(1 * time.Second) // Back off on error
			continue
		}

		// Process messages
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				event := q.parseMessage(msg)

				// Call handler
				if err := handler(ctx, event); err != nil {
					log.Error().Err(err).Str("id", msg.ID).Msg("Handler error")
					q.failed.Add(1)
					// Don't ack - let it be reprocessed
					continue
				}

				// Acknowledge successful processing
				if err := q.client.XAck(ctx, q.stream, q.consumerGroup, msg.ID).Err(); err != nil {
					log.Warn().Err(err).Str("id", msg.ID).Msg("Failed to acknowledge message")
				}

				q.processed.Add(1)
				q.lastEvent.Store(time.Now())
			}
		}
	}
}

func (q *redisQueue) parseMessage(msg redis.XMessage) *Event {
	event := &Event{
		ID:        msg.ID,
		Timestamp: time.Now(), // Redis doesn't store original timestamp in simple case
	}

	// Parse fields
	if v, ok := msg.Values["type"].(string); ok {
		event.Type = v
	}
	if v, ok := msg.Values["ansName"].(string); ok {
		event.ANSName = v
	}
	if v, ok := msg.Values["fqdn"].(string); ok {
		event.FQDN = v
	}
	if v, ok := msg.Values["version"].(string); ok {
		event.Version = v
	}
	if v, ok := msg.Values["registrarId"].(string); ok {
		event.RegistrarID = v
	}
	if v, ok := msg.Values["signature"].(string); ok {
		event.Signature = v
	}

	// Parse metadata if present
	if v, ok := msg.Values["meta"].(string); ok {
		if err := json.Unmarshal([]byte(v), &event.Meta); err != nil {
			log.Warn().Err(err).Msg("Failed to unmarshal event metadata")
		}
	}

	return event
}

// Publish sends an event to the Redis stream
func (q *redisQueue) Publish(ctx context.Context, event *Event) error {
	if q.closed.Load() {
		return ErrQueueClosed
	}

	values := map[string]interface{}{
		"type":        event.Type,
		"ansName":     event.ANSName,
		"fqdn":        event.FQDN,
		"version":     event.Version,
		"registrarId": event.RegistrarID,
		"signature":   event.Signature,
		"timestamp":   event.Timestamp.Unix(),
	}

	if event.Meta.Description != "" || len(event.Meta.Capabilities) > 0 {
		metaJSON, _ := json.Marshal(event.Meta)
		values["meta"] = string(metaJSON)
	}

	_, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// Acknowledge confirms event processing
func (q *redisQueue) Acknowledge(ctx context.Context, eventID string) error {
	return q.client.XAck(ctx, q.stream, q.consumerGroup, eventID).Err()
}

// Reject marks an event as failed
func (q *redisQueue) Reject(ctx context.Context, eventID string, requeue bool) error {
	if requeue {
		// Move to a dead-letter stream or requeue
		// For now, just don't acknowledge
		return nil
	}
	// Acknowledge to remove from pending
	return q.Acknowledge(ctx, eventID)
}

// Stats returns queue statistics
func (q *redisQueue) Stats(ctx context.Context) (*Stats, error) {
	// Get stream info
	info, err := q.client.XInfoStream(ctx, q.stream).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	// Get pending count for this consumer group
	pending, err := q.client.XPending(ctx, q.stream, q.consumerGroup).Result()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get pending info")
	}

	var pendingCount int64
	if pending != nil {
		pendingCount = pending.Count
	}

	lastEvent := q.lastEvent.Load().(time.Time)

	return &Stats{
		Pending:       pendingCount,
		Processed:     q.processed.Load(),
		Failed:        q.failed.Load(),
		ConsumerLag:   info.Length - q.processed.Load(),
		LastEventTime: lastEvent,
	}, nil
}

// Close releases resources
func (q *redisQueue) Close() error {
	if q.closed.CompareAndSwap(false, true) {
		q.wg.Wait()
		return q.client.Close()
	}
	return nil
}

// Name returns the provider name
func (q *redisQueue) Name() string {
	return "redis-streams"
}

func init() {
	Register("redis-streams", func(opts Options, config map[string]interface{}) (Provider, error) {
		redisOpts := DefaultRedisQueueOptions()

		if v, ok := config["address"].(string); ok {
			redisOpts.Address = v
		}
		if v, ok := config["password"].(string); ok {
			redisOpts.Password = v
		}
		if v, ok := config["db"].(int); ok {
			redisOpts.DB = v
		}

		redisOpts.ConsumerName = opts.ConsumerName
		redisOpts.BlockTimeout = opts.BlockTimeout
		redisOpts.BatchSize = int64(opts.BatchSize)

		return NewRedisQueue(redisOpts)
	})
}
