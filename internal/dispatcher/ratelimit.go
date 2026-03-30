package dispatcher

import (
	"context"
	"sync"
	"time"
)

// RateLimiter throttles dispatch execution with a global rate limit and
// per-rule cooldowns. Thread-safe for concurrent use.
type RateLimiter struct {
	mu         sync.Mutex
	global     time.Duration
	lastGlobal time.Time
	cooldowns  map[string]time.Time // rule name -> last dispatch time
}

// NewRateLimiter creates a rate limiter with the given global minimum interval
// between dispatches. Pass 0 for no global rate limit.
func NewRateLimiter(global time.Duration) *RateLimiter {
	return &RateLimiter{
		global:    global,
		cooldowns: make(map[string]time.Time),
	}
}

// Wait blocks until the global rate limit allows the next dispatch. Returns
// an error if the context is canceled while waiting.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if rl.global <= 0 {
		return nil
	}

	rl.mu.Lock()
	elapsed := time.Since(rl.lastGlobal)
	rl.mu.Unlock()

	if elapsed >= rl.global {
		return nil
	}

	remaining := rl.global - elapsed
	select {
	case <-time.After(remaining):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AllowRule checks if a rule's cooldown has expired. Returns true if the rule
// is allowed to fire, false if it's still in cooldown.
func (rl *RateLimiter) AllowRule(ruleName string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	last, ok := rl.cooldowns[ruleName]
	if !ok {
		return true
	}
	return time.Since(last) >= cooldown
}

// Record marks the current time as the last dispatch for both the global rate
// limiter and the named rule.
func (rl *RateLimiter) Record(ruleName string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rl.lastGlobal = now
	rl.cooldowns[ruleName] = now
}
