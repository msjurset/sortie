package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestReloaderCurrent(t *testing.T) {
	cfg := &Config{LogDir: "/test"}
	r := NewReloader(cfg, "/fake/config.yaml", testLogger())

	got := r.Current()
	if got.LogDir != "/test" {
		t.Errorf("Current().LogDir = %q, want %q", got.LogDir, "/test")
	}
}

func TestReloaderReload(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Write initial config
	initial := `log_dir: /initial
rules:
  - name: rule1
    match:
      extensions: [.txt]
    action:
      type: move
      dest: /dest
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReloader(cfg, cfgPath, testLogger())

	if len(r.Current().Rules) != 1 {
		t.Fatalf("initial rules = %d, want 1", len(r.Current().Rules))
	}

	// Update config
	updated := `log_dir: /updated
rules:
  - name: rule1
    match:
      extensions: [.txt]
    action:
      type: move
      dest: /dest
  - name: rule2
    match:
      extensions: [.pdf]
    action:
      type: copy
      dest: /dest2
`
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := r.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	if len(r.Current().Rules) != 2 {
		t.Errorf("after reload rules = %d, want 2", len(r.Current().Rules))
	}
}

func TestReloaderReloadInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Write valid initial config
	if err := os.WriteFile(cfgPath, []byte("log_dir: /test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReloader(cfg, cfgPath, testLogger())

	// Write invalid YAML
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = r.Reload()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}

	// Original config should still be accessible
	if r.Current().LogDir != "/test" {
		t.Error("original config should be preserved after failed reload")
	}
}

func TestReloaderConcurrentReads(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(cfgPath, []byte("log_dir: /test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReloader(cfg, cfgPath, testLogger())

	// Concurrent reads should not panic
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := r.Current()
			_ = c.LogDir
		}()
	}

	// Reload concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.Reload()
	}()

	wg.Wait()
}

func TestReloaderWatchDetectsChange(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	initial := `rules:
  - name: rule1
    match:
      extensions: [.txt]
    action:
      type: move
      dest: /dest
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	r := NewReloader(cfg, cfgPath, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching in background
	go r.Watch(ctx, nil)

	// Give fsnotify time to start
	time.Sleep(100 * time.Millisecond)

	// Update config
	updated := `rules:
  - name: rule1
    match:
      extensions: [.txt]
    action:
      type: move
      dest: /dest
  - name: rule2
    match:
      extensions: [.pdf]
    action:
      type: copy
      dest: /dest2
`
	if err := os.WriteFile(cfgPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + reload
	time.Sleep(800 * time.Millisecond)

	if len(r.Current().Rules) != 2 {
		t.Errorf("after watch reload rules = %d, want 2", len(r.Current().Rules))
	}

	cancel()
}

func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		path    string
		central string
		want    bool
	}{
		{"/home/user/.config/sortie/config.yaml", "/home/user/.config/sortie/config.yaml", true},
		{"/home/user/Downloads/.sortie.yaml", "/other/config.yaml", true},
		{"/home/user/Downloads/report.pdf", "/other/config.yaml", false},
		{"/home/user/Downloads/config.yaml", "/other/config.yaml", false},
	}

	for _, tt := range tests {
		got := isConfigFile(tt.path, tt.central)
		if got != tt.want {
			t.Errorf("isConfigFile(%q, %q) = %v, want %v", tt.path, tt.central, got, tt.want)
		}
	}
}
