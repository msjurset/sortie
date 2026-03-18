package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
	"gopkg.in/yaml.v3"
)

// Config holds the central dispatch configuration.
type Config struct {
	LogDir      string      `yaml:"log_dir"`
	HistoryFile string      `yaml:"history_file"`
	TrashDir    string      `yaml:"trash_dir"`
	Directories []Directory `yaml:"directories"`
	Rules       []rule.Rule `yaml:"rules"`
}

// Directory represents a watched directory entry.
type Directory struct {
	Path      string `yaml:"path"`
	Recursive bool   `yaml:"recursive"`
}

// DirConfig holds per-directory rules from a .sortie.yaml file.
type DirConfig struct {
	Rules []rule.Rule `yaml:"rules"`
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sortie", "config.yaml")
}

// Load reads the config from disk. If path is empty, the default path is used.
// If the file doesn't exist, a default config is returned.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	path = expandHome(path)

	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.LogDir = expandHome(cfg.LogDir)
	cfg.HistoryFile = expandHome(cfg.HistoryFile)
	cfg.TrashDir = expandHome(cfg.TrashDir)
	for i := range cfg.Directories {
		cfg.Directories[i].Path = expandHome(cfg.Directories[i].Path)
	}
	for i := range cfg.Rules {
		cfg.Rules[i].Action.Dest = expandHome(cfg.Rules[i].Action.Dest)
	}

	return cfg, nil
}

// LoadDirConfig reads a .sortie.yaml from the given directory.
// Returns nil with no error if the file doesn't exist.
func LoadDirConfig(dir string) (*DirConfig, error) {
	path := filepath.Join(dir, ".sortie.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var dc DirConfig
	if err := yaml.Unmarshal(data, &dc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	for i := range dc.Rules {
		dc.Rules[i].Action.Dest = expandHome(dc.Rules[i].Action.Dest)
	}

	return &dc, nil
}

// MergedRules returns the combined rules for a directory: per-directory rules
// first (higher priority), then global rules.
func (c *Config) MergedRules(dir string) ([]rule.Rule, error) {
	dc, err := LoadDirConfig(dir)
	if err != nil {
		return nil, err
	}

	var merged []rule.Rule
	if dc != nil {
		merged = append(merged, dc.Rules...)
	}
	merged = append(merged, c.Rules...)
	return merged, nil
}

// EnsureDirs creates the necessary directories.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		filepath.Dir(c.HistoryFile),
		c.LogDir,
		c.TrashDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}
	return nil
}

func defaults() *Config {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".config", "sortie")
	return &Config{
		LogDir:      filepath.Join(base, "logs"),
		HistoryFile: filepath.Join(base, "history.json"),
		TrashDir:    filepath.Join(base, "trash"),
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
