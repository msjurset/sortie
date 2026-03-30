package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// tempSuffixes are file extensions used by browsers/downloaders for incomplete files.
var tempSuffixes = []string{
	".crdownload", ".part", ".partial", ".download", ".tmp",
}

// Watcher monitors directories for new files and calls a handler after a
// debounce period.
type Watcher struct {
	fsw      *fsnotify.Watcher
	debounce time.Duration
	mu       sync.Mutex
	timers   map[string]*time.Timer
	handler  func(path string)
	logger   *slog.Logger
}

// New creates a Watcher that monitors the given directories.
func New(dirs []string, debounce time.Duration, logger *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if err := fsw.Add(dir); err != nil {
			fsw.Close()
			return nil, err
		}
	}

	return &Watcher{
		fsw:      fsw,
		debounce: debounce,
		timers:   make(map[string]*time.Timer),
		logger:   logger,
	}, nil
}

// Run starts the event loop. It blocks until the context is canceled.
func (w *Watcher) Run(ctx context.Context, handler func(path string)) error {
	w.handler = handler

	for {
		select {
		case <-ctx.Done():
			w.drainTimers()
			return w.fsw.Close()

		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("watcher error", "err", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
		return
	}

	path := event.Name
	name := filepath.Base(path)

	// Skip directories
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}

	// Skip dotfiles
	if strings.HasPrefix(name, ".") {
		return
	}

	// Skip temp/partial download files
	for _, suffix := range tempSuffixes {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return
		}
	}

	w.resetTimer(path)
}

func (w *Watcher) resetTimer(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}

	w.timers[path] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()

		// Verify file still exists (it may have been moved/deleted)
		if _, err := os.Stat(path); err != nil {
			return
		}

		w.handler(path)
	})
}

func (w *Watcher) drainTimers() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path, t := range w.timers {
		t.Stop()
		delete(w.timers, path)
	}
}
