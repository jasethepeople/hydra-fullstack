package orchestrator

import (
	"errors"
	"sync"
	"time"

	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
)

// CircuitState represents the circuit breaker state
type CircuitState int

const (
	StateClosed    CircuitState = 0 // Normal operation
	StateOpen      CircuitState = 1 // Failing, reject requests
	StateHalfOpen  CircuitState = 2 // Testing if service recovered
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu                sync.RWMutex
	serviceName       string
	state             CircuitState
	failureCount      int
	successCount      int
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	lastFailureTime   time.Time
	logger            *logging.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(serviceName string, failureThreshold, successThreshold int, timeout time.Duration, logger *logging.Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		serviceName:      serviceName,
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		logger:           logger,
	}
	metrics.UpdateCircuitBreaker(serviceName, 0)
	return cb
}

// Call executes the function if circuit allows
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition from Open to Half-Open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.state = StateHalfOpen
		cb.failureCount = 0
		cb.successCount = 0
		cb.logger.CircuitBreakerState(cb.serviceName, "half-open", cb.failureCount)
		metrics.UpdateCircuitBreaker(cb.serviceName, 2)
	}

	// Reject if circuit is open
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return errors.New("circuit breaker is open for " + cb.serviceName)
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.recordResult(err)
	return err
}

// recordResult updates circuit state based on success/failure
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.state == StateHalfOpen {
			// Failed during test, go back to open
			cb.state = StateOpen
			cb.logger.CircuitBreakerState(cb.serviceName, "open", cb.failureCount)
			metrics.UpdateCircuitBreaker(cb.serviceName, 1)
		} else if cb.failureCount >= cb.failureThreshold {
			// Too many failures, open circuit
			cb.state = StateOpen
			cb.logger.CircuitBreakerState(cb.serviceName, "open", cb.failureCount)
			metrics.UpdateCircuitBreaker(cb.serviceName, 1)
		}
	} else {
		cb.successCount++

		if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {
			// Service recovered, close circuit
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.logger.CircuitBreakerState(cb.serviceName, "closed", 0)
			metrics.UpdateCircuitBreaker(cb.serviceName, 0)
		} else if cb.state == StateClosed && cb.successCount > 0 {
			// Reset failure count on success in closed state
			if cb.failureCount > 0 {
				cb.failureCount = 0
			}
		}
	}
}

// GetState returns current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return map[string]interface{}{
		"service":        cb.serviceName,
		"state":          cb.state.String(),
		"failure_count":  cb.failureCount,
		"success_count":  cb.successCount,
		"last_failure":   cb.lastFailureTime.Format(time.RFC3339),
		"timeout":        cb.timeout.String(),
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu        sync.RWMutex
	breakers  map[string]*CircuitBreaker
	logger    *logging.Logger
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry(logger *logging.Logger) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
	}
}

// GetOrCreate returns existing or creates new circuit breaker
func (r *CircuitBreakerRegistry) GetOrCreate(serviceName string, failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, exists := r.breakers[serviceName]; exists {
		return cb
	}

	cb := NewCircuitBreaker(serviceName, failureThreshold, successThreshold, timeout, r.logger)
	r.breakers[serviceName] = cb
	return cb
}

// Get returns existing circuit breaker
func (r *CircuitBreakerRegistry) Get(serviceName string) (*CircuitBreaker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cb, exists := r.breakers[serviceName]
	return cb, exists
}

// GetAllStats returns stats for all circuits
func (r *CircuitBreakerRegistry) GetAllStats() map[string]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, cb := range r.breakers {
		stats[name] = cb.GetStats()
	}
	return stats
}
