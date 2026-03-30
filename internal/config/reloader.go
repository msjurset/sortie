package config

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Reloader wraps a Config with thread-safe hot-reload capability. It watches
// config files for changes and swaps in the new config atomically. Readers get
// a consistent snapshot via Current(); in-flight operations are not interrupted.
type Reloader struct {
	mu      sync.RWMutex
	cfg     *Config
	cfgPath string
	logger  *slog.Logger
}

// NewReloader creates a reloader initialized with the given config.
func NewReloader(cfg *Config, cfgPath string, logger *slog.Logger) *Reloader {
	return &Reloader{
		cfg:     cfg,
		cfgPath: cfgPath,
		logger:  logger,
	}
}

// Current returns the current config snapshot. The returned pointer is safe to
// use without holding the lock — it points to an immutable snapshot.
func (r *Reloader) Current() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

// Reload loads the config from disk, validates it, and swaps it in if valid.
func (r *Reloader) Reload() error {
	newCfg, err := Load(r.cfgPath)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.cfg = newCfg
	r.mu.Unlock()

	return nil
}

// Watch monitors config files for changes and reloads automatically. It watches
// the central config file and each directory's .sortie.yaml. Changes are
// debounced to avoid rapid reloads from editors that write multiple times.
// Blocks until the context is canceled.
func (r *Reloader) Watch(ctx context.Context, dirPaths []string) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Watch the central config file's directory (fsnotify watches dirs, not files)
	if err := fsw.Add(dirParent(r.cfgPath)); err != nil {
		r.logger.Warn("cannot watch config dir", "path", r.cfgPath, "err", err)
	}

	// Watch each directory for .sortie.yaml changes
	for _, dir := range dirPaths {
		if err := fsw.Add(dir); err != nil {
			r.logger.Warn("cannot watch directory for config changes", "path", dir, "err", err)
		}
	}

	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	resetDebounce := func() {
		debounceMu.Lock()
		defer debounceMu.Unlock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
			if err := r.Reload(); err != nil {
				r.logger.Error("config reload failed", "err", err)
			} else {
				cfg := r.Current()
				r.logger.Info("config reloaded", "rules", len(cfg.Rules))
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceMu.Unlock()
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			// Only react to config files
			if isConfigFile(event.Name, r.cfgPath) {
				resetDebounce()
			}

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			r.logger.Error("config watcher error", "err", err)
		}
	}
}

func isConfigFile(path, centralPath string) bool {
	name := baseName(path)
	return path == centralPath || name == ".sortie.yaml"
}

func dirParent(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
