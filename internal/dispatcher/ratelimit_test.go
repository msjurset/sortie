package dispatcher

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterNoLimit(t *testing.T) {
	rl := NewRateLimiter(0)
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
}

func TestRateLimiterGlobalThrottle(t *testing.T) {
	rl := NewRateLimiter(100 * time.Millisecond)

	rl.Record("rule1")

	start := time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 80*time.Millisecond {
		t.Errorf("Wait() returned too fast: %v (expected ~100ms)", elapsed)
	}
}

func TestRateLimiterGlobalNoWaitAfterExpiry(t *testing.T) {
	rl := NewRateLimiter(10 * time.Millisecond)
	rl.Record("rule1")

	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("Wait() should return immediately after expiry, took %v", elapsed)
	}
}

func TestRateLimiterCancelDuringWait(t *testing.T) {
	rl := NewRateLimiter(1 * time.Second)
	rl.Record("rule1")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected context error from cancelled Wait()")
	}
}

func TestRateLimiterAllowRule(t *testing.T) {
	rl := NewRateLimiter(0)

	// First call — no prior record, should allow
	if !rl.AllowRule("upload", 1*time.Second) {
		t.Error("first call should be allowed")
	}

	// Record dispatch
	rl.Record("upload")

	// Immediately after — should be in cooldown
	if rl.AllowRule("upload", 1*time.Second) {
		t.Error("should be in cooldown")
	}

	// Different rule — should be allowed
	if !rl.AllowRule("move", 1*time.Second) {
		t.Error("different rule should not be affected by upload's cooldown")
	}
}

func TestRateLimiterAllowRuleAfterExpiry(t *testing.T) {
	rl := NewRateLimiter(0)
	rl.Record("upload")

	time.Sleep(20 * time.Millisecond)

	if !rl.AllowRule("upload", 10*time.Millisecond) {
		t.Error("should be allowed after cooldown expires")
	}
}

func TestRateLimiterZeroCooldownAlwaysAllows(t *testing.T) {
	rl := NewRateLimiter(0)
	rl.Record("rule1")

	if !rl.AllowRule("rule1", 0) {
		t.Error("zero cooldown should always allow")
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(1 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rl.Record("rule1")
			rl.AllowRule("rule1", 1*time.Millisecond)
			rl.Wait(context.Background())
		}(i)
	}
	wg.Wait()
}
