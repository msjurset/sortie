package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
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
