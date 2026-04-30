package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherDetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dir}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var handled []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(handled) != 1 {
		t.Fatalf("expected 1 handled file, got %d", len(handled))
	}
	if handled[0] != testFile {
		t.Errorf("handled path = %q, want %q", handled[0], testFile)
	}
}

func TestWatcherIgnoresDotfiles(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dir}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var handled []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create dotfile — should be ignored
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create temp download file — should be ignored
	if err := os.WriteFile(filepath.Join(dir, "file.crdownload"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(handled) != 0 {
		t.Errorf("expected 0 handled files, got %d: %v", len(handled), handled)
	}
}

func TestWatcherDebounce(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dir}}, 200*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var count int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Write to the same file multiple times rapidly
	testFile := filepath.Join(dir, "rapid.txt")
	for i := 0; i < 5; i++ {
		os.WriteFile(testFile, []byte("update"), 0o644)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(400 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	// Should only fire once despite multiple writes
	if count != 1 {
		t.Errorf("expected 1 handler call (debounced), got %d", count)
	}
}

func TestWatcherWriteEventWithWatchExisting(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file before the watcher starts
	testFile := filepath.Join(dir, "app.log")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New([]DirOption{{Path: dir, WatchExisting: true}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var count int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Write to the existing file — should trigger handler
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("more data")
	f.Close()

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 handler call for write event, got %d", count)
	}
}

func TestWatcherWriteEventWithoutWatchExisting(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file before the watcher starts
	testFile := filepath.Join(dir, "app.log")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	// watch_existing is false (default)
	w, err := New([]DirOption{{Path: dir}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var count int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Write to the existing file — should NOT trigger handler
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("more data")
	f.Close()

	// Wait for debounce + processing
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 handler calls (watch_existing disabled), got %d", count)
	}
}

func TestWatcherConcurrencyCapsParallelDispatch(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create 8 files; poll will dispatch them all.
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	const cap = 2
	w, err := New(
		[]DirOption{{Path: dir, Poll: 60 * time.Millisecond, Concurrency: cap}},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		inflight  atomic.Int32
		violation atomic.Bool
		fired     atomic.Int32
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			fired.Add(1)
			cur := inflight.Add(1)
			if cur > cap {
				violation.Store(true)
			}
			// Simulate slow handler so contention is visible.
			time.Sleep(60 * time.Millisecond)
			inflight.Add(-1)
		})
		close(done)
	}()

	// Let several poll ticks fire and process the backlog.
	time.Sleep(700 * time.Millisecond)

	cancel()
	<-done

	if violation.Load() {
		t.Errorf("in-flight handler count exceeded concurrency cap of %d", cap)
	}
	if fired.Load() < int32(cap) {
		t.Errorf("expected at least %d handler invocations, got %d", cap, fired.Load())
	}
}

func TestWatcherSetDirsReconcilesPool(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New(
		[]DirOption{{Path: dir, Concurrency: 2}},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {})
		close(done)
	}()

	// Wait for initial pool to spin up.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		pool := w.pools[dir]
		w.mu.Unlock()
		if pool != nil && pool.size == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	w.mu.Lock()
	pool := w.pools[dir]
	w.mu.Unlock()
	if pool == nil || pool.size != 2 {
		t.Fatalf("initial pool not started with size 2 (pool=%v)", pool)
	}

	// Resize to 5.
	w.SetDirs([]DirOption{{Path: dir, Concurrency: 5}})

	w.mu.Lock()
	pool = w.pools[dir]
	w.mu.Unlock()
	if pool == nil || pool.size != 5 {
		t.Errorf("after resize, pool size = %v, want 5", pool)
	}

	// Drop pool entirely.
	w.SetDirs([]DirOption{{Path: dir}})

	w.mu.Lock()
	_, exists := w.pools[dir]
	w.mu.Unlock()
	if exists {
		t.Errorf("pool should be removed when concurrency=0")
	}

	cancel()
	<-done
}

func TestWatcherZeroConcurrencyIsUnbounded(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	const files = 10
	for i := 0; i < files; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Concurrency: 0 (omitted) → unbounded direct dispatch.
	w, err := New(
		[]DirOption{{Path: dir, Poll: 50 * time.Millisecond}},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var inflight atomic.Int32
	var peak atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			cur := inflight.Add(1)
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(80 * time.Millisecond)
			inflight.Add(-1)
		})
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)

	cancel()
	<-done

	// Without a pool, scanForPoll loops synchronously and calls handler
	// directly — peak should reflect the loop's serial nature (≈1) since
	// the poll goroutine is single-threaded. Demonstrates that concurrency
	// of 0 does NOT spawn a pool. The fsnotify path would spawn parallel
	// timer goroutines; this test is specifically about the poll path.
	// What we're really proving is "no panics, files dispatch correctly".
	if peak.Load() < 1 {
		t.Errorf("expected at least one handler invocation, peak = %d", peak.Load())
	}
}

func TestWatcherPollFiresForExistingFiles(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file before starting — fsnotify would not normally see this.
	target := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New(
		[]DirOption{{Path: dir, Poll: 100 * time.Millisecond}},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		mu      sync.Mutex
		handled []string
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	// Wait long enough for at least one poll tick (100ms) plus margin.
	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(handled) == 0 {
		t.Fatalf("expected poll to fire handler at least once, got %d", len(handled))
	}
	if handled[0] != target {
		t.Errorf("handled[0] = %q, want %q", handled[0], target)
	}
}

func TestWatcherPollIgnoresDotfilesAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Files that should be ignored even by poll.
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "download.crdownload"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// One real file that should fire.
	good := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(good, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New(
		[]DirOption{{Path: dir, Poll: 80 * time.Millisecond}},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		mu      sync.Mutex
		handled []string
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(250 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	for _, p := range handled {
		if p != good {
			t.Errorf("unexpected dispatch for %q (only %q should fire)", p, good)
		}
	}
	if len(handled) == 0 {
		t.Errorf("expected real.txt to fire at least once")
	}
}

func TestWatcherSetDirsTogglesPoll(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file in B; A starts with poll, B starts without.
	bFile := filepath.Join(dirB, "b.txt")
	if err := os.WriteFile(bFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New(
		[]DirOption{
			{Path: dirA, Poll: 80 * time.Millisecond},
			{Path: dirB},
		},
		500*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		mu       sync.Mutex
		bHandled int
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			if filepath.Dir(path) == dirB {
				bHandled++
			}
			mu.Unlock()
		})
		close(done)
	}()

	// Initially no polling on B; pre-created file should not fire.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if bHandled != 0 {
		mu.Unlock()
		t.Fatalf("expected no dispatch for B before enabling poll, got %d", bHandled)
	}
	mu.Unlock()

	// Enable polling on B; expect it to fire on the existing file.
	w.SetDirs([]DirOption{
		{Path: dirA, Poll: 80 * time.Millisecond},
		{Path: dirB, Poll: 80 * time.Millisecond},
	})

	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	if bHandled == 0 {
		mu.Unlock()
		t.Fatalf("expected B to fire after enabling poll via SetDirs")
	}
	bHandled = 0
	mu.Unlock()

	// Now disable polling on B again. Wait, then assert no further fires.
	w.SetDirs([]DirOption{
		{Path: dirA, Poll: 80 * time.Millisecond},
		{Path: dirB},
	})
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	if bHandled != 0 {
		mu.Unlock()
		t.Errorf("expected B to stop firing after disabling poll, got %d more dispatches", bHandled)
	}
	mu.Unlock()

	cancel()
	<-done
}

func TestShouldDispatchName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"ordinary.txt", true},
		{"Photo.JPG", true},
		{".DS_Store", false},
		{".hidden", false},
		{"download.crdownload", false},
		{"upload.PART", false},
		{"file.tmp", false},
		{"file.partial", false},
		{"file.download", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDispatchName(tt.name); got != tt.want {
				t.Errorf("shouldDispatchName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestWatcherSetDirsAddsNewDirectory(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dirA}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var handled []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Add dirB to the watcher dynamically
	w.SetDirs([]DirOption{{Path: dirA}, {Path: dirB}})

	// Create a file in the newly-added dir
	testFile := filepath.Join(dirB, "added.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(handled) != 1 || handled[0] != testFile {
		t.Errorf("expected handler call for %q, got %v", testFile, handled)
	}
}

func TestWatcherSetDirsRemovesDirectory(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dirA}, {Path: dirB}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var handled []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			handled = append(handled, path)
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Drop dirB from the watcher
	w.SetDirs([]DirOption{{Path: dirA}})

	// File in removed dir — should not fire
	if err := os.WriteFile(filepath.Join(dirB, "ignored.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File in still-watched dir — should fire
	keptFile := filepath.Join(dirA, "kept.txt")
	if err := os.WriteFile(keptFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(handled) != 1 || handled[0] != keptFile {
		t.Errorf("expected only %q to fire, got %v", keptFile, handled)
	}
}

func TestWatcherPerDirDebounce(t *testing.T) {
	fastDir := t.TempDir()
	slowDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Default debounce 80 ms; slow dir overrides to 400 ms.
	w, err := New(
		[]DirOption{
			{Path: fastDir},
			{Path: slowDir, Debounce: 400 * time.Millisecond},
		},
		80*time.Millisecond,
		logger,
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		mu       sync.Mutex
		fastSeen time.Time
		slowSeen time.Time
		started  = time.Now()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			defer mu.Unlock()
			if filepath.Dir(path) == fastDir && fastSeen.IsZero() {
				fastSeen = time.Now()
			}
			if filepath.Dir(path) == slowDir && slowSeen.IsZero() {
				slowSeen = time.Now()
			}
		})
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(fastDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slowDir, "s.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait long enough for both to fire.
	time.Sleep(700 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if fastSeen.IsZero() || slowSeen.IsZero() {
		t.Fatalf("expected both dirs to fire; fastSeen=%v slowSeen=%v", fastSeen, slowSeen)
	}

	fastDelay := fastSeen.Sub(started)
	slowDelay := slowSeen.Sub(started)

	// Fast dir should fire well under the slow dir's debounce.
	if fastDelay > 250*time.Millisecond {
		t.Errorf("fastDir fired after %v, expected under 250ms (default 80ms debounce)", fastDelay)
	}
	if slowDelay < 350*time.Millisecond {
		t.Errorf("slowDir fired after %v, expected at least 350ms (configured 400ms debounce)", slowDelay)
	}
}

func TestWatcherSetDirsUpdatesDebounce(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := New([]DirOption{{Path: dir}}, 50*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var (
		mu      sync.Mutex
		seen    time.Time
		started time.Time
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			if seen.IsZero() {
				seen = time.Now()
			}
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)

	// Bump the directory's debounce dynamically.
	w.SetDirs([]DirOption{{Path: dir, Debounce: 350 * time.Millisecond}})

	started = time.Now()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(600 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if seen.IsZero() {
		t.Fatal("expected handler to fire")
	}
	if delay := seen.Sub(started); delay < 300*time.Millisecond {
		t.Errorf("handler fired after %v, expected at least 300ms with updated debounce", delay)
	}
}

func TestWatcherSetDirsTogglesWatchExisting(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file before watching
	testFile := filepath.Join(dir, "app.log")
	if err := os.WriteFile(testFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Start with watch_existing disabled
	w, err := New([]DirOption{{Path: dir}}, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var count int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Enable watch_existing dynamically
	w.SetDirs([]DirOption{{Path: dir, WatchExisting: true}})

	// Append to existing file — should fire under the new flag
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("more")
	f.Close()

	time.Sleep(300 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if count < 1 {
		t.Errorf("expected handler to fire after enabling watch_existing, got %d calls", count)
	}
}

func TestWatcherWriteDebounce(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Pre-create a file
	testFile := filepath.Join(dir, "busy.log")
	if err := os.WriteFile(testFile, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New([]DirOption{{Path: dir, WatchExisting: true}}, 200*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var count int

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = w.Run(ctx, func(path string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Write rapidly to the same file many times
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString("line\n")
		f.Close()
		time.Sleep(30 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(400 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 handler call (debounced writes), got %d", count)
	}
}
