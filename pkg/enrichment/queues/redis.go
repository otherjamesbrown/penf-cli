package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisQueue implements Queue using Redis lists and sorted sets.
type RedisQueue struct {
	client     *redis.Client
	name       string
	config     QueueConfig
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewRedisQueue creates a new Redis-backed queue.
func NewRedisQueue(client *redis.Client, config QueueConfig) *RedisQueue {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisQueue{
		client:     client,
		name:       config.Name,
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// Redis key prefixes
const (
	keyPrefixQueue      = "queue:"      // Main queue (sorted set by priority)
	keyPrefixProcessing = "processing:" // Messages being processed
	keyPrefixMessage    = "msg:"        // Message data
	keyPrefixDLQ        = "dlq:"        // Dead letter queue
)

// Name returns the queue name.
func (q *RedisQueue) Name() string {
	return q.name
}

// Enqueue adds a message to the queue.
func (q *RedisQueue) Enqueue(msg Message) error {
	return q.enqueueSingle(msg)
}

func (q *RedisQueue) enqueueSingle(msg Message) error {
	// Generate message ID
	messageID := uuid.New().String()

	// Serialize the message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create queued message wrapper
	qm := &QueuedMessage{
		ID:          messageID,
		Message:     msgBytes,
		MessageType: msg.GetMessageType(),
		Priority:    msg.GetPriority(),
		RetryCount:  0,
		EnqueuedAt:  time.Now(),
	}

	qmBytes, err := json.Marshal(qm)
	if err != nil {
		return fmt.Errorf("failed to marshal queued message: %w", err)
	}

	// Store message data and add to sorted set in a transaction
	pipe := q.client.TxPipeline()

	// Store message data with TTL
	msgKey := keyPrefixMessage + q.name + ":" + messageID
	pipe.Set(q.ctx, msgKey, qmBytes, q.config.RetentionPeriod)

	// Add to queue sorted set (score = priority * 1e12 + timestamp for FIFO within priority)
	queueKey := keyPrefixQueue + q.name
	score := float64(msg.GetPriority())*1e12 + float64(time.Now().UnixNano())
	pipe.ZAdd(q.ctx, queueKey, redis.Z{Score: score, Member: messageID})

	_, err = pipe.Exec(q.ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	return nil
}

// EnqueueBatch adds multiple messages to the queue.
func (q *RedisQueue) EnqueueBatch(msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}

	pipe := q.client.TxPipeline()
	queueKey := keyPrefixQueue + q.name
	now := time.Now()

	for _, msg := range msgs {
		messageID := uuid.New().String()

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		qm := &QueuedMessage{
			ID:          messageID,
			Message:     msgBytes,
			MessageType: msg.GetMessageType(),
			Priority:    msg.GetPriority(),
			RetryCount:  0,
			EnqueuedAt:  now,
		}

		qmBytes, err := json.Marshal(qm)
		if err != nil {
			return fmt.Errorf("failed to marshal queued message: %w", err)
		}

		msgKey := keyPrefixMessage + q.name + ":" + messageID
		pipe.Set(q.ctx, msgKey, qmBytes, q.config.RetentionPeriod)

		score := float64(msg.GetPriority())*1e12 + float64(now.UnixNano())
		pipe.ZAdd(q.ctx, queueKey, redis.Z{Score: score, Member: messageID})
	}

	_, err := pipe.Exec(q.ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	return nil
}

// Dequeue retrieves messages from the queue.
func (q *RedisQueue) Dequeue(maxMessages int, timeout time.Duration) ([]*QueuedMessage, error) {
	if maxMessages <= 0 {
		maxMessages = 1
	}

	queueKey := keyPrefixQueue + q.name
	processingKey := keyPrefixProcessing + q.name
	deadline := time.Now().Add(timeout)

	var messages []*QueuedMessage

	for time.Now().Before(deadline) && len(messages) < maxMessages {
		// Pop highest priority message (highest score)
		result, err := q.client.ZPopMax(q.ctx, queueKey, 1).Result()
		if err == redis.Nil || len(result) == 0 {
			// Queue is empty, wait a bit and retry
			select {
			case <-time.After(100 * time.Millisecond):
				continue
			case <-q.ctx.Done():
				return messages, q.ctx.Err()
			}
		}
		if err != nil {
			return messages, fmt.Errorf("failed to pop from queue: %w", err)
		}

		messageID := result[0].Member.(string)
		msgKey := keyPrefixMessage + q.name + ":" + messageID

		// Get message data
		data, err := q.client.Get(q.ctx, msgKey).Bytes()
		if err == redis.Nil {
			// Message expired, skip
			continue
		}
		if err != nil {
			return messages, fmt.Errorf("failed to get message data: %w", err)
		}

		var qm QueuedMessage
		if err := json.Unmarshal(data, &qm); err != nil {
			return messages, fmt.Errorf("failed to unmarshal message: %w", err)
		}

		// Move to processing set with visibility timeout
		visibleAfter := time.Now().Add(q.config.VisibilityTimeout)
		qm.VisibleAfter = visibleAfter

		// Update message data with visibility info
		updatedData, _ := json.Marshal(qm)
		pipe := q.client.TxPipeline()
		pipe.Set(q.ctx, msgKey, updatedData, q.config.RetentionPeriod)
		pipe.ZAdd(q.ctx, processingKey, redis.Z{
			Score:  float64(visibleAfter.UnixNano()),
			Member: messageID,
		})
		if _, err := pipe.Exec(q.ctx); err != nil {
			return messages, fmt.Errorf("failed to move to processing: %w", err)
		}

		messages = append(messages, &qm)
	}

	return messages, nil
}

// Ack acknowledges successful processing of a message.
func (q *RedisQueue) Ack(messageID string) error {
	processingKey := keyPrefixProcessing + q.name
	msgKey := keyPrefixMessage + q.name + ":" + messageID

	pipe := q.client.TxPipeline()
	pipe.ZRem(q.ctx, processingKey, messageID)
	pipe.Del(q.ctx, msgKey)
	_, err := pipe.Exec(q.ctx)
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}

	return nil
}

// Nack indicates processing failure, message will be retried.
func (q *RedisQueue) Nack(messageID string) error {
	processingKey := keyPrefixProcessing + q.name
	msgKey := keyPrefixMessage + q.name + ":" + messageID

	// Get current message data
	data, err := q.client.Get(q.ctx, msgKey).Bytes()
	if err == redis.Nil {
		return ErrMessageNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	var qm QueuedMessage
	if err := json.Unmarshal(data, &qm); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Increment retry count
	qm.RetryCount++

	if qm.RetryCount >= q.config.MaxRetries {
		// Move to dead letter queue
		return q.MoveToDeadLetter(messageID, "max retries exceeded")
	}

	// Re-enqueue with backoff
	queueKey := keyPrefixQueue + q.name
	backoff := calculateBackoff(qm.RetryCount)
	qm.VisibleAfter = time.Now().Add(backoff)

	updatedData, _ := json.Marshal(qm)

	pipe := q.client.TxPipeline()
	pipe.ZRem(q.ctx, processingKey, messageID)
	pipe.Set(q.ctx, msgKey, updatedData, q.config.RetentionPeriod)
	// Re-add to queue with delayed visibility
	score := float64(qm.Priority)*1e12 + float64(qm.VisibleAfter.UnixNano())
	pipe.ZAdd(q.ctx, queueKey, redis.Z{Score: score, Member: messageID})

	_, err = pipe.Exec(q.ctx)
	if err != nil {
		return fmt.Errorf("failed to nack message: %w", err)
	}

	return nil
}

// MoveToDeadLetter moves a message to the dead letter queue.
func (q *RedisQueue) MoveToDeadLetter(messageID string, reason string) error {
	processingKey := keyPrefixProcessing + q.name
	msgKey := keyPrefixMessage + q.name + ":" + messageID
	dlqKey := keyPrefixDLQ + q.name

	// Get message data
	data, err := q.client.Get(q.ctx, msgKey).Bytes()
	if err == redis.Nil {
		return ErrMessageNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Store in DLQ with reason
	dlqEntry := map[string]interface{}{
		"message":    string(data),
		"reason":     reason,
		"moved_at":   time.Now().Format(time.RFC3339),
		"queue_name": q.name,
	}
	dlqData, _ := json.Marshal(dlqEntry)

	pipe := q.client.TxPipeline()
	pipe.ZRem(q.ctx, processingKey, messageID)
	pipe.Del(q.ctx, msgKey)
	pipe.ZAdd(q.ctx, dlqKey, redis.Z{
		Score:  float64(time.Now().UnixNano()),
		Member: string(dlqData),
	})

	_, err = pipe.Exec(q.ctx)
	if err != nil {
		return fmt.Errorf("failed to move to DLQ: %w", err)
	}

	return nil
}

// Depth returns the current queue depth.
func (q *RedisQueue) Depth() (int64, error) {
	queueKey := keyPrefixQueue + q.name
	return q.client.ZCard(q.ctx, queueKey).Result()
}

// Close closes the queue connection.
func (q *RedisQueue) Close() error {
	q.cancelFunc()
	return nil
}

// calculateBackoff calculates exponential backoff for retries.
func calculateBackoff(retryCount int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, etc., max 5 minutes
	base := time.Second
	backoff := base * (1 << uint(retryCount))
	maxBackoff := 5 * time.Minute
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}

// RecoverStaleMessages recovers messages that exceeded visibility timeout.
// Should be called periodically by a background worker.
func (q *RedisQueue) RecoverStaleMessages() error {
	processingKey := keyPrefixProcessing + q.name
	queueKey := keyPrefixQueue + q.name

	// Find messages whose visibility timeout has expired
	now := float64(time.Now().UnixNano())
	staleMessages, err := q.client.ZRangeByScore(q.ctx, processingKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", now),
		Count: 100,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to find stale messages: %w", err)
	}

	for _, messageID := range staleMessages {
		msgKey := keyPrefixMessage + q.name + ":" + messageID

		// Get message to check retry count
		data, err := q.client.Get(q.ctx, msgKey).Bytes()
		if err == redis.Nil {
			// Message expired, just remove from processing
			q.client.ZRem(q.ctx, processingKey, messageID)
			continue
		}
		if err != nil {
			continue
		}

		var qm QueuedMessage
		if err := json.Unmarshal(data, &qm); err != nil {
			continue
		}

		qm.RetryCount++

		if qm.RetryCount >= q.config.MaxRetries {
			q.MoveToDeadLetter(messageID, "visibility timeout exceeded")
			continue
		}

		// Re-enqueue
		updatedData, _ := json.Marshal(qm)
		pipe := q.client.TxPipeline()
		pipe.ZRem(q.ctx, processingKey, messageID)
		pipe.Set(q.ctx, msgKey, updatedData, q.config.RetentionPeriod)
		score := float64(qm.Priority)*1e12 + float64(time.Now().UnixNano())
		pipe.ZAdd(q.ctx, queueKey, redis.Z{Score: score, Member: messageID})
		pipe.Exec(q.ctx)
	}

	return nil
}

// Verify interface compliance
var _ Queue = (*RedisQueue)(nil)
