package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
	"hydra-fullstack/internal/queue"
	"hydra-fullstack/internal/storage"
)

// WorkerPool manages a pool of goroutine workers
type WorkerPool struct {
	mu           sync.RWMutex
	workers      []*Worker
	queue        *queue.RedisQueue
	store        *storage.Store
	cfg          *config.QueueConfig
	logger       *logging.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	circuitBreakers *CircuitBreakerRegistry
}

// Worker represents a single worker goroutine
type Worker struct {
	ID       int
	pool     *WorkerPool
	active   bool
	busy     bool
	ctx      context.Context
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(q *queue.RedisQueue, store *storage.Store, cfg *config.QueueConfig, logger *logging.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		queue:           q,
		store:           store,
		cfg:             cfg,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		circuitBreakers: NewCircuitBreakerRegistry(logger),
	}
}

// Start initializes and starts all workers
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for i := 0; i < wp.cfg.WorkerCount; i++ {
		worker := &Worker{
			ID:   i,
			pool: wp,
			ctx:  wp.ctx,
		}
		wp.workers = append(wp.workers, worker)
		wp.wg.Add(1)
		go worker.run()
	}

	metrics.UpdateWorkers(len(wp.workers), 0)
	log.Info().Int("worker_count", len(wp.workers)).Msg("worker pool started")
}

// Stop gracefully shuts down all workers
func (wp *WorkerPool) Stop() {
	log.Info().Msg("stopping worker pool...")
	wp.cancel()
	wp.wg.Wait()
	log.Info().Msg("worker pool stopped")
}

// GetStats returns current worker pool statistics
func (wp *WorkerPool) GetStats() map[string]interface{} {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	active := 0
	busy := 0
	for _, w := range wp.workers {
		if w.active {
			active++
		}
		if w.busy {
			busy++
		}
	}

	return map[string]interface{}{
		"total_workers":   len(wp.workers),
		"active_workers":  active,
		"busy_workers":    busy,
		"idle_workers":    active - busy,
		"queue_depth":     wp.queue.GetDepth(),
		"dead_letter":     wp.queue.GetDeadLetterCount(),
	}
}

// run is the main worker loop
func (w *Worker) run() {
	defer w.pool.wg.Done()
	w.active = true

	w.pool.logger.WorkerEvent("worker_started", w.ID, w.pool.queue.GetDepth())

	for {
		select {
		case <-w.ctx.Done():
			w.active = false
			w.pool.logger.WorkerEvent("worker_stopped", w.ID, 0)
			return
		default:
			w.processJob()
		}
	}
}

// processJob dequeues and executes a single job
func (w *Worker) processJob() {
	job, err := w.pool.queue.Dequeue(5 * time.Second)
	if err != nil {
		log.Error().Err(err).Int("worker_id", w.ID).Msg("failed to dequeue job")
		return
	}
	if job == nil {
		return // No job available
	}

	w.busy = true
	metrics.UpdateWorkers(len(w.pool.workers), w.pool.getBusyCount())
	w.pool.logger.WorkerEvent("worker_busy", w.ID, w.pool.queue.GetDepth())

	start := time.Now()
	jobID := uuid.New().String()

	// Save job to database
	w.pool.store.SaveRenderJob(jobID, "processing", job.Width, job.Height, job.Complexity, job.Iterations)

	// Execute with circuit breaker protection
	discordCB := w.pool.circuitBreakers.GetOrCreate("discord", 5, 3, 30*time.Second, w.pool.logger)

	err = discordCB.Call(func() error {
		return w.executeRender(job)
	})

	duration := time.Since(start)

	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Int("worker_id", w.ID).Msg("render failed")

		w.pool.store.CompleteRenderJob(jobID, int(duration.Milliseconds()), err.Error())
		metrics.RecordRender("failed", job.Complexity, duration)

		// Requeue with backoff
		backoff := time.Duration(job.RetryCount+1) * time.Second
		w.pool.queue.Requeue(job, backoff)
	} else {
		log.Info().Str("job_id", jobID).Dur("duration", duration).Int("worker_id", w.ID).Msg("render completed")

		w.pool.store.CompleteRenderJob(jobID, int(duration.Milliseconds()), "")
		metrics.RecordRender("completed", job.Complexity, duration)
	}

	w.busy = false
	metrics.UpdateWorkers(len(w.pool.workers), w.pool.getBusyCount())
	w.pool.logger.WorkerEvent("worker_idle", w.ID, w.pool.queue.GetDepth())
}

// executeRender performs the actual render work
func (w *Worker) executeRender(job *queue.Job) error {
	w.pool.logger.RenderStarted(job.ID, job.Width, job.Height, job.Complexity)

	// Simulate render work based on complexity
	var workDuration time.Duration
	switch job.Complexity {
	case "low":
		workDuration = time.Duration(job.Iterations) * time.Millisecond
	case "medium":
		workDuration = time.Duration(job.Iterations*2) * time.Millisecond
	case "high":
		workDuration = time.Duration(job.Iterations*5) * time.Millisecond
	default:
		workDuration = time.Duration(job.Iterations) * time.Millisecond
	}

	// Cap at reasonable max
	if workDuration > 30*time.Second {
		workDuration = 30 * time.Second
	}

	// Simulate memory allocation
	bufferSize := job.Width * job.Height * 4
	if bufferSize > 0 {
		_ = make([]byte, bufferSize)
	}

	// Do the work
	time.Sleep(workDuration)

	// Simulate occasional failures for testing
	if job.Iterations > 8000 {
		return fmt.Errorf("render timeout: exceeded maximum allowed time")
	}

	w.pool.logger.RenderCompleted(job.ID, workDuration, true)
	return nil
}

// getBusyCount returns count of busy workers
func (wp *WorkerPool) getBusyCount() int {
	count := 0
	for _, w := range wp.workers {
		if w.busy {
			count++
		}
	}
	return count
}

// SubmitJob submits a new job to the queue
func (wp *WorkerPool) SubmitJob(jobType string, width, height int, complexity string, iterations int, priority int) (string, error) {
	job := &queue.Job{
		ID:         uuid.New().String(),
		Type:       jobType,
		Width:      width,
		Height:     height,
		Complexity: complexity,
		Iterations: iterations,
		Priority:   priority,
	}

	if err := wp.queue.Enqueue(job); err != nil {
		return "", fmt.Errorf("failed to submit job: %w", err)
	}

	log.Info().Str("job_id", job.ID).Str("type", jobType).Int("priority", priority).Msg("job submitted")
	return job.ID, nil
}
