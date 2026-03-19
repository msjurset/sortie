package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msjurset/sortie/internal/rule"
)

func TestDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.LogDir == "" {
		t.Error("LogDir should not be empty")
	}
	if cfg.HistoryFile == "" {
		t.Error("HistoryFile should not be empty")
	}
	if cfg.TrashDir == "" {
		t.Error("TrashDir should not be empty")
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() should not error on missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() should return default config on missing file")
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
directories:
  - path: /tmp/test-downloads
    recursive: false

rules:
  - name: test-rule
    match:
      extensions: [.txt]
    action:
      type: move
      dest: /tmp/test-dest
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Directories) != 1 {
		t.Fatalf("expected 1 directory, got %d", len(cfg.Directories))
	}
	if cfg.Directories[0].Path != "/tmp/test-downloads" {
		t.Errorf("directory path = %q, want %q", cfg.Directories[0].Path, "/tmp/test-downloads")
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "test-rule" {
		t.Errorf("rule name = %q, want %q", cfg.Rules[0].Name, "test-rule")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(cfgPath, []byte(":::invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("Load() should error on invalid YAML")
	}
}

func TestLoadDirConfigMissing(t *testing.T) {
	dc, err := LoadDirConfig(t.TempDir())
	if err != nil {
		t.Fatalf("LoadDirConfig() should not error on missing file: %v", err)
	}
	if dc != nil {
		t.Error("LoadDirConfig() should return nil on missing file")
	}
}

func TestLoadDirConfigValid(t *testing.T) {
	dir := t.TempDir()
	content := `
rules:
  - name: local-rule
    match:
      extensions: [.pdf]
    action:
      type: move
      dest: /tmp/pdfs
`
	if err := os.WriteFile(filepath.Join(dir, ".sortie.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	dc, err := LoadDirConfig(dir)
	if err != nil {
		t.Fatalf("LoadDirConfig() error: %v", err)
	}
	if dc == nil {
		t.Fatal("LoadDirConfig() returned nil")
	}
	if len(dc.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(dc.Rules))
	}
	if dc.Rules[0].Name != "local-rule" {
		t.Errorf("rule name = %q, want %q", dc.Rules[0].Name, "local-rule")
	}
}

func TestMergedRules(t *testing.T) {
	dir := t.TempDir()

	// Write a per-dir config
	content := `
rules:
  - name: local-first
    match:
      extensions: [.pdf]
    action:
      type: move
      dest: /tmp/local
`
	if err := os.WriteFile(filepath.Join(dir, ".sortie.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := defaults()
	cfg.Rules = []rule.Rule{
		{Name: "global-second", Action: rule.Action{Type: "move", Dest: "/tmp/global"}},
	}

	merged, err := cfg.MergedRules(dir)
	if err != nil {
		t.Fatalf("MergedRules() error: %v", err)
	}

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged rules, got %d", len(merged))
	}

	// Per-dir rules should come first
	if merged[0].Name != "local-first" {
		t.Errorf("first rule = %q, want %q (per-dir should be first)", merged[0].Name, "local-first")
	}
	if merged[1].Name != "global-second" {
		t.Errorf("second rule = %q, want %q", merged[1].Name, "global-second")
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		LogDir:      filepath.Join(dir, "logs"),
		HistoryFile: filepath.Join(dir, "data", "history.json"),
		TrashDir:    filepath.Join(dir, "trash"),
	}

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error: %v", err)
	}

	for _, d := range []string{cfg.LogDir, filepath.Dir(cfg.HistoryFile), cfg.TrashDir} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %q should exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("%q should be a directory", d)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
