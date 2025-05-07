package utils

import (
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerWrapper wraps API calls with circuit breaker functionality
type CircuitBreakerWrapper struct {
	cb *gobreaker.CircuitBreaker
}

// NewCircuitBreaker creates a new circuit breaker with specified settings
func NewCircuitBreaker(name string) *CircuitBreakerWrapper {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	}

	return &CircuitBreakerWrapper{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

// Execute executes the given function with circuit breaker protection
func (cbw *CircuitBreakerWrapper) Execute(fn func() (interface{}, error)) (interface{}, error) {
	return cbw.cb.Execute(fn)
}
