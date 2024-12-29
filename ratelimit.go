package etcd

import (
	"context"
	"golang.org/x/time/rate"
)

// rateLimiter wraps the golang.org/x/time/rate limiter
type rateLimiter struct {
	limiter *rate.Limiter
}

// newRateLimiter creates a new rate limiter with the specified rate and burst
func newRateLimiter(r float64, b int) *rateLimiter {
	return &rateLimiter{
		limiter: rate.NewLimiter(rate.Limit(r), b),
	}
}

// wait blocks until the rate limit allows an event or the context is cancelled
func (r *rateLimiter) wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}
