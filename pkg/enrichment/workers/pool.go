// Package workers provides worker pool management for the enrichment pipeline.
package workers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/queues"
)

// WorkerType identifies the type of worker.
type WorkerType string

const (
	WorkerTypeIngest     WorkerType = "ingest"
	WorkerTypeEnrichment WorkerType = "enrichment"
	WorkerTypeAI         WorkerType = "ai"
)

// WorkerStatus represents the worker's current status.
type WorkerStatus string

const (
	WorkerStatusStarting  WorkerStatus = "starting"
	WorkerStatusHealthy   WorkerStatus = "healthy"
	WorkerStatusUnhealthy WorkerStatus = "unhealthy"
	WorkerStatusDraining  WorkerStatus = "draining"
	WorkerStatusStopped   WorkerStatus = "stopped"
)

// MessageHandler processes a queue message.
type MessageHandler func(ctx context.Context, msg queues.Message) error

// WorkerConfig configures a worker.
type WorkerConfig struct {
	WorkerType        WorkerType    `yaml:"worker_type"`
	Count             int           `yaml:"count"`
	QueueName         string        `yaml:"queue_name"`
	BatchSize         int           `yaml:"batch_size"`
	VisibilityTimeout time.Duration `yaml:"visibility_timeout"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
}

// DefaultWorkerConfigs returns default worker configurations.
func DefaultWorkerConfigs() map[WorkerType]WorkerConfig {
	return map[WorkerType]WorkerConfig{
		WorkerTypeIngest: {
			WorkerType:        WorkerTypeIngest,
			Count:             4,
			QueueName:         "enrichment:ingest",
			BatchSize:         10,
			VisibilityTimeout: 60 * time.Second,
			PollInterval:      1 * time.Second,
			ShutdownTimeout:   30 * time.Second,
		},
		WorkerTypeEnrichment: {
			WorkerType:        WorkerTypeEnrichment,
			Count:             8,
			QueueName:         "enrichment:process",
			BatchSize:         1,
			VisibilityTimeout: 120 * time.Second,
			PollInterval:      500 * time.Millisecond,
			ShutdownTimeout:   60 * time.Second,
		},
		WorkerTypeAI: {
			WorkerType:        WorkerTypeAI,
			Count:             4,
			QueueName:         "enrichment:ai",
			BatchSize:         1,
			VisibilityTimeout: 300 * time.Second,
			PollInterval:      1 * time.Second,
			ShutdownTimeout:   120 * time.Second,
		},
	}
}

// Worker represents a single worker processing messages.
type Worker struct {
	ID          string
	Type        WorkerType
	Config      WorkerConfig
	Status      WorkerStatus
	Queue       queues.Queue
	Handler     MessageHandler
	StartedAt   time.Time
	LastActivity time.Time

	// Metrics
	ProcessedCount atomic.Int64
	FailedCount    atomic.Int64

	// Control
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         *sync.WaitGroup
}

// NewWorker creates a new worker.
func NewWorker(config WorkerConfig, queue queues.Queue, handler MessageHandler) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		ID:         uuid.New().String(),
		Type:       config.WorkerType,
		Config:     config,
		Status:     WorkerStatusStarting,
		Queue:      queue,
		Handler:    handler,
		ctx:        ctx,
		cancelFunc: cancel,
		wg:         &sync.WaitGroup{},
	}
}

// Start begins processing messages.
func (w *Worker) Start() {
	w.StartedAt = time.Now()
	w.Status = WorkerStatusHealthy
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		w.processLoop()
	}()
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	w.Status = WorkerStatusDraining
	w.cancelFunc()

	// Wait for shutdown with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.Status = WorkerStatusStopped
	case <-time.After(w.Config.ShutdownTimeout):
		w.Status = WorkerStatusStopped
	}
}

func (w *Worker) processLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			messages, err := w.Queue.Dequeue(w.Config.BatchSize, w.Config.PollInterval)
			if err != nil {
				if err == w.ctx.Err() {
					return
				}
				// Log error and continue
				time.Sleep(w.Config.PollInterval)
				continue
			}

			for _, qm := range messages {
				if w.ctx.Err() != nil {
					return
				}

				w.processMessage(qm)
			}
		}
	}
}

func (w *Worker) processMessage(qm *queues.QueuedMessage) {
	w.LastActivity = time.Now()

	msg, err := qm.ParseMessage()
	if err != nil {
		// Invalid message, move to DLQ
		w.Queue.MoveToDeadLetter(qm.ID, fmt.Sprintf("parse error: %v", err))
		w.FailedCount.Add(1)
		return
	}

	// Process with timeout
	ctx, cancel := context.WithTimeout(w.ctx, w.Config.VisibilityTimeout-10*time.Second)
	defer cancel()

	err = w.Handler(ctx, msg)
	if err != nil {
		// Check error type for retry decision
		if procErr, ok := err.(*queues.ProcessingError); ok {
			if procErr.IsRetryable() {
				w.Queue.Nack(qm.ID)
			} else {
				w.Queue.MoveToDeadLetter(qm.ID, procErr.Error())
			}
		} else {
			// Unknown error, retry
			w.Queue.Nack(qm.ID)
		}
		w.FailedCount.Add(1)
		return
	}

	// Success
	w.Queue.Ack(qm.ID)
	w.ProcessedCount.Add(1)
}

// Pool manages a pool of workers.
type Pool struct {
	Type    WorkerType
	Config  WorkerConfig
	Workers []*Worker
	Queue   queues.Queue
	Handler MessageHandler

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPool creates a new worker pool.
func NewPool(config WorkerConfig, queue queues.Queue, handler MessageHandler) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		Type:    config.WorkerType,
		Config:  config,
		Queue:   queue,
		Handler: handler,
		Workers: make([]*Worker, 0, config.Count),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts all workers in the pool.
func (p *Pool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < p.Config.Count; i++ {
		worker := NewWorker(p.Config, p.Queue, p.Handler)
		worker.Start()
		p.Workers = append(p.Workers, worker)
	}
}

// Stop gracefully stops all workers.
func (p *Pool) Stop() {
	p.cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	var wg sync.WaitGroup
	for _, worker := range p.Workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			w.Stop()
		}(worker)
	}
	wg.Wait()
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PoolStats{
		Type:         p.Type,
		WorkerCount:  len(p.Workers),
		ActiveCount:  0,
		Processed:    0,
		Failed:       0,
	}

	for _, w := range p.Workers {
		if w.Status == WorkerStatusHealthy {
			stats.ActiveCount++
		}
		stats.Processed += w.ProcessedCount.Load()
		stats.Failed += w.FailedCount.Load()
	}

	return stats
}

// PoolStats contains pool statistics.
type PoolStats struct {
	Type         WorkerType
	WorkerCount  int
	ActiveCount  int
	Processed    int64
	Failed       int64
}

// PoolManager manages multiple worker pools.
type PoolManager struct {
	pools  map[WorkerType]*Pool
	mu     sync.RWMutex
}

// NewPoolManager creates a new pool manager.
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[WorkerType]*Pool),
	}
}

// RegisterPool registers a worker pool.
func (pm *PoolManager) RegisterPool(pool *Pool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.pools[pool.Type] = pool
}

// GetPool returns a pool by type.
func (pm *PoolManager) GetPool(workerType WorkerType) (*Pool, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pool, ok := pm.pools[workerType]
	return pool, ok
}

// StartAll starts all registered pools.
func (pm *PoolManager) StartAll() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, pool := range pm.pools {
		pool.Start()
	}
}

// StopAll stops all registered pools.
func (pm *PoolManager) StopAll() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var wg sync.WaitGroup
	for _, pool := range pm.pools {
		wg.Add(1)
		go func(p *Pool) {
			defer wg.Done()
			p.Stop()
		}(pool)
	}
	wg.Wait()
}

// AllStats returns statistics for all pools.
func (pm *PoolManager) AllStats() map[WorkerType]PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make(map[WorkerType]PoolStats)
	for workerType, pool := range pm.pools {
		stats[workerType] = pool.Stats()
	}
	return stats
}
