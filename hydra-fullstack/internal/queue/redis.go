package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
)

// Job represents a render job in the queue
type Job struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Complexity string    `json:"complexity"`
	Iterations int       `json:"iterations"`
	Payload    string    `json:"payload,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Priority   int       `json:"priority"`
	RetryCount int       `json:"retry_count"`
}

// RedisQueue implements a priority job queue with Redis
type RedisQueue struct {
	client     *redis.Client
	queueName  string
	dlQueue    string
	logger     *logging.Logger
	ctx        context.Context
}

// NewRedisQueue creates a new Redis-backed queue
func NewRedisQueue(cfg *config.QueueConfig, logger *logging.Logger) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		PoolSize: 10,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	q := &RedisQueue{
		client:    client,
		queueName: cfg.QueueName,
		dlQueue:   cfg.DeadLetterName,
		logger:    logger,
		ctx:       ctx,
	}

	log.Info().Str("addr", cfg.RedisAddr).Str("queue", cfg.QueueName).Msg("redis queue initialized")
	return q, nil
}

// Enqueue adds a job to the queue with priority
func (q *RedisQueue) Enqueue(job *Job) error {
	job.CreatedAt = time.Now().UTC()
	if job.Status == "" {
		job.Status = "pending"
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Use sorted set with priority score (lower = higher priority)
	score := float64(job.Priority) + float64(time.Now().UnixNano())/1e15

	pipe := q.client.Pipeline()
	pipe.ZAdd(q.ctx, q.queueName, redis.Z{Score: score, Member: string(data)})
	pipe.Incr(q.ctx, q.queueName+":count")
	_, err = pipe.Exec(q.ctx)

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	q.updateDepth()
	q.logger.WorkerEvent("job_enqueued", 0, q.GetDepth())
	return nil
}

// Dequeue retrieves the highest priority job
func (q *RedisQueue) Dequeue(timeout time.Duration) (*Job, error) {
	// Use ZPOPMIN to get highest priority (lowest score)
	result, err := q.client.ZPopMin(q.ctx, q.queueName, 1).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		// No jobs available, block and wait
		return q.blockingDequeue(timeout)
	}

	var job Job
	if err := json.Unmarshal([]byte(result[0].Member.(string)), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	job.Status = "processing"
	q.updateDepth()
	q.logger.WorkerEvent("job_dequeued", 0, q.GetDepth())
	return &job, nil
}

// blockingDequeue waits for new jobs using BZPOPMIN
func (q *RedisQueue) blockingDequeue(timeout time.Duration) (*Job, error) {
	result, err := q.client.BZPopMin(q.ctx, timeout, q.queueName).Result()
	if err == redis.Nil {
		return nil, nil // Timeout, no job available
	}
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(result.Member.(string)), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	job.Status = "processing"
	q.updateDepth()
	q.logger.WorkerEvent("job_dequeued", 0, q.GetDepth())
	return &job, nil
}

// Requeue returns a failed job to the queue with incremented retry count
func (q *RedisQueue) Requeue(job *Job, delay time.Duration) error {
	job.RetryCount++
	job.Status = "pending"

	if job.RetryCount > 3 {
		// Move to dead letter queue
		return q.moveToDeadLetter(job)
	}

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	// Delayed requeue - add delay to score
	score := float64(job.Priority) + float64(time.Now().Add(delay).UnixNano())/1e15

	_, err = q.client.ZAdd(q.ctx, q.queueName, redis.Z{Score: score, Member: string(data)}).Result()
	if err != nil {
		return fmt.Errorf("failed to requeue job: %w", err)
	}

	q.updateDepth()
	q.logger.WorkerEvent("job_requeued", 0, q.GetDepth())
	return nil
}

// moveToDeadLetter moves a job to the dead letter queue
func (q *RedisQueue) moveToDeadLetter(job *Job) error {
	job.Status = "dead_letter"
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	_, err = q.client.LPush(q.ctx, q.dlQueue, string(data)).Result()
	if err != nil {
		return fmt.Errorf("failed to move to dead letter: %w", err)
	}

	q.logger.WorkerEvent("job_dead_letter", 0, q.GetDepth())
	log.Warn().Str("job_id", job.ID).Int("retries", job.RetryCount).Msg("job moved to dead letter queue")
	return nil
}

// GetDepth returns current queue depth
func (q *RedisQueue) GetDepth() int64 {
	depth, err := q.client.ZCard(q.ctx, q.queueName).Result()
	if err != nil {
		return 0
	}
	return depth
}

// updateDepth updates the metrics gauge
func (q *RedisQueue) updateDepth() {
	metrics.UpdateQueueDepth(q.GetDepth())
}

// GetDeadLetterCount returns count of dead letter jobs
func (q *RedisQueue) GetDeadLetterCount() int64 {
	count, err := q.client.LLen(q.ctx, q.dlQueue).Result()
	if err != nil {
		return 0
	}
	return count
}

// Peek returns jobs without removing them
func (q *RedisQueue) Peek(limit int) ([]*Job, error) {
	results, err := q.client.ZRange(q.ctx, q.queueName, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	jobs := make([]*Job, 0, len(results))
	for _, data := range results {
		var job Job
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			continue
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// Purge removes all jobs from the queue
func (q *RedisQueue) Purge() error {
	_, err := q.client.Del(q.ctx, q.queueName).Result()
	return err
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}
