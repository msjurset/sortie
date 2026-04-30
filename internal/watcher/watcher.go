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

// DirOption holds per-directory settings for the watcher.
type DirOption struct {
	Path          string
	WatchExisting bool
	// Debounce overrides the watcher's default debounce for this directory.
	// Zero means inherit the default. Useful for slow-syncing mounts where
	// files may not be fully materialized when fsnotify fires.
	Debounce time.Duration
	// Poll, when non-zero, runs a periodic directory walk on this interval
	// in addition to fsnotify-based event watching. Each tick lists the
	// directory and dispatches every regular file to the handler. Useful
	// for cloud-storage mounts where fsnotify events are unreliable.
	Poll time.Duration
	// Concurrency caps the number of handler invocations that may run in
	// parallel for files in this directory. Zero means unbounded (every
	// dispatch spawns its own goroutine, the original behavior).
	Concurrency int
}

// pollEntry tracks a running per-directory poller.
type pollEntry struct {
	cancel   context.CancelFunc
	interval time.Duration
}

// workerPool tracks a per-directory bounded worker pool that serializes
// handler invocations to at most `size` concurrent calls.
type workerPool struct {
	queue  chan string
	cancel context.CancelFunc
	size   int
}

// Watcher monitors directories for new files and calls a handler after a
// debounce period. It also supports per-directory periodic polling for
// mounts where fsnotify is unreliable, and per-directory bounded
// concurrency to cap parallel handler invocations during bursts.
type Watcher struct {
	fsw            *fsnotify.Watcher
	debounce       time.Duration
	mu             sync.Mutex
	timers         map[string]*time.Timer
	watchExisting  map[string]bool
	dirDebounce    map[string]time.Duration
	dirPoll        map[string]time.Duration
	dirConcurrency map[string]int
	pollers        map[string]*pollEntry
	pools          map[string]*workerPool
	runCtx         context.Context // shared by pollers and worker pools; set by Run
	handler        func(path string)
	logger         *slog.Logger
}

// New creates a Watcher that monitors the given directories.
func New(dirs []DirOption, debounce time.Duration, logger *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	we := make(map[string]bool)
	dd := make(map[string]time.Duration)
	dp := make(map[string]time.Duration)
	dc := make(map[string]int)
	for _, d := range dirs {
		if err := fsw.Add(d.Path); err != nil {
			fsw.Close()
			return nil, err
		}
		if d.WatchExisting {
			we[d.Path] = true
		}
		if d.Debounce > 0 {
			dd[d.Path] = d.Debounce
		}
		if d.Poll > 0 {
			dp[d.Path] = d.Poll
		}
		if d.Concurrency > 0 {
			dc[d.Path] = d.Concurrency
		}
	}

	return &Watcher{
		fsw:            fsw,
		debounce:       debounce,
		timers:         make(map[string]*time.Timer),
		watchExisting:  we,
		dirDebounce:    dd,
		dirPoll:        dp,
		dirConcurrency: dc,
		pollers:        make(map[string]*pollEntry),
		pools:          make(map[string]*workerPool),
		logger:         logger,
	}, nil
}

// SetDirs reconciles the watcher's subscribed directories with the given list.
// New paths are added, paths no longer present are removed, and watch_existing
// flags are refreshed. Errors adding or removing paths are logged.
func (w *Watcher) SetDirs(dirs []DirOption) {
	w.mu.Lock()
	defer w.mu.Unlock()

	next := make(map[string]DirOption, len(dirs))
	for _, d := range dirs {
		next[d.Path] = d
	}

	current := make(map[string]bool)
	for _, p := range w.fsw.WatchList() {
		current[p] = true
	}

	for path, opt := range next {
		if !current[path] {
			if err := w.fsw.Add(path); err != nil {
				w.logger.Warn("watcher add failed", "path", path, "err", err)
				continue
			}
			w.logger.Info("watcher added directory", "path", path)
		}
		if opt.WatchExisting {
			w.watchExisting[path] = true
		} else {
			delete(w.watchExisting, path)
		}
		if opt.Debounce > 0 {
			w.dirDebounce[path] = opt.Debounce
		} else {
			delete(w.dirDebounce, path)
		}
		w.reconcilePollerLocked(path, opt.Poll)
		w.reconcilePoolLocked(path, opt.Concurrency)
	}

	for path := range current {
		if _, keep := next[path]; !keep {
			if err := w.fsw.Remove(path); err != nil {
				w.logger.Warn("watcher remove failed", "path", path, "err", err)
			} else {
				w.logger.Info("watcher removed directory", "path", path)
			}
			delete(w.watchExisting, path)
			delete(w.dirDebounce, path)
			w.reconcilePollerLocked(path, 0)
			w.reconcilePoolLocked(path, 0)
		}
	}
}

// reconcilePollerLocked starts, stops, or restarts the poller for path so it
// matches the desired interval. Caller must hold w.mu. A zero interval stops
// any running poller. Restarting (when the interval changes) cancels the old
// goroutine and starts a fresh one. No-op if runCtx isn't set yet (Run hasn't
// started); the initial pollers are spawned by Run itself.
func (w *Watcher) reconcilePollerLocked(path string, interval time.Duration) {
	if interval > 0 {
		w.dirPoll[path] = interval
	} else {
		delete(w.dirPoll, path)
	}

	if w.runCtx == nil {
		return
	}

	existing, running := w.pollers[path]
	if !running && interval > 0 {
		w.startPollerLocked(path, interval)
		return
	}
	if running && interval == 0 {
		existing.cancel()
		delete(w.pollers, path)
		w.logger.Info("watcher stopped poller", "path", path)
		return
	}
	if running && interval != existing.interval {
		existing.cancel()
		delete(w.pollers, path)
		w.startPollerLocked(path, interval)
		w.logger.Info("watcher restarted poller", "path", path, "interval", interval)
	}
}

// startPollerLocked spawns a goroutine that walks path every interval and
// invokes the handler for each dispatch-eligible file. Caller must hold w.mu
// and ensure w.runCtx is set.
func (w *Watcher) startPollerLocked(path string, interval time.Duration) {
	ctx, cancel := context.WithCancel(w.runCtx)
	w.pollers[path] = &pollEntry{cancel: cancel, interval: interval}
	go w.pollLoop(ctx, path, interval)
	w.logger.Info("watcher started poller", "path", path, "interval", interval)
}

// reconcilePoolLocked starts, stops, or restarts the worker pool for path so
// it matches the desired size. Caller must hold w.mu. A zero size stops the
// pool (dispatches fall back to direct, unbounded handler calls). Resizing
// drains the existing queue's in-flight items via context cancellation and
// spawns a fresh pool. No-op until Run sets w.runCtx.
func (w *Watcher) reconcilePoolLocked(path string, size int) {
	if size > 0 {
		w.dirConcurrency[path] = size
	} else {
		delete(w.dirConcurrency, path)
	}

	if w.runCtx == nil {
		return
	}

	existing, running := w.pools[path]
	if !running && size > 0 {
		w.startPoolLocked(path, size)
		return
	}
	if running && size == 0 {
		existing.cancel()
		delete(w.pools, path)
		w.logger.Info("watcher stopped pool", "path", path)
		return
	}
	if running && size != existing.size {
		existing.cancel()
		delete(w.pools, path)
		w.startPoolLocked(path, size)
		w.logger.Info("watcher restarted pool", "path", path, "concurrency", size)
	}
}

// startPoolLocked creates a per-directory queue and spawns `size` worker
// goroutines that drain it into the handler. Caller must hold w.mu and
// ensure w.runCtx is set. Workers exit when ctx is canceled.
func (w *Watcher) startPoolLocked(path string, size int) {
	ctx, cancel := context.WithCancel(w.runCtx)
	queue := make(chan string, size)
	w.pools[path] = &workerPool{queue: queue, cancel: cancel, size: size}
	for i := 0; i < size; i++ {
		go w.workerLoop(ctx, queue)
	}
	w.logger.Info("watcher started pool", "path", path, "concurrency", size)
}

// workerLoop consumes paths from queue and invokes the handler. Exits when
// ctx is canceled or the queue is closed.
func (w *Watcher) workerLoop(ctx context.Context, queue <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-queue:
			if !ok {
				return
			}
			if w.handler != nil {
				w.handler(path)
			}
		}
	}
}

// dispatch routes a path to the appropriate handler invocation: through the
// directory's bounded worker pool if one is configured, or directly (a fresh
// goroutine inheriting the original unbounded behavior) if not. Send is
// non-blocking on shutdown via a select on runCtx.
func (w *Watcher) dispatch(path string) {
	if w.handler == nil {
		return
	}
	dir := filepath.Dir(path)
	w.mu.Lock()
	pool := w.pools[dir]
	ctx := w.runCtx
	w.mu.Unlock()
	if pool == nil {
		w.handler(path)
		return
	}
	select {
	case pool.queue <- path:
	case <-ctx.Done():
	}
}

// Run starts the event loop. It blocks until the context is canceled. Any
// directories configured with a poll interval get a poller goroutine spawned
// here; pollers exit when ctx is canceled.
func (w *Watcher) Run(ctx context.Context, handler func(path string)) error {
	w.handler = handler

	w.mu.Lock()
	w.runCtx = ctx
	for path, size := range w.dirConcurrency {
		w.startPoolLocked(path, size)
	}
	for path, interval := range w.dirPoll {
		w.startPollerLocked(path, interval)
	}
	w.mu.Unlock()

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

// pollLoop walks dir every interval and dispatches each eligible file to the
// handler. Designed for cloud-storage mounts where fsnotify is unreliable.
func (w *Watcher) pollLoop(ctx context.Context, dir string, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.scanForPoll(dir)
		}
	}
}

func (w *Watcher) scanForPoll(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.logger.Warn("poll readdir failed", "dir", dir, "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !shouldDispatchName(e.Name()) {
			continue
		}
		w.dispatch(filepath.Join(dir, e.Name()))
	}
}

// shouldDispatchName filters out names that should never be dispatched
// regardless of source (fsnotify event or poll walk): dotfiles and partial
// download files (.crdownload, .part, .tmp, etc.).
func shouldDispatchName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return false
	}
	lower := strings.ToLower(name)
	for _, suffix := range tempSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	return true
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	isCreate := event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
	isWrite := event.Has(fsnotify.Write)

	if !isCreate && !isWrite {
		return
	}

	path := event.Name

	// For write events, only proceed if the directory has watch_existing enabled
	if isWrite && !isCreate {
		dir := filepath.Dir(path)
		w.mu.Lock()
		we := w.watchExisting[dir]
		w.mu.Unlock()
		if !we {
			return
		}
	}

	// Skip directories
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}

	if !shouldDispatchName(filepath.Base(path)) {
		return
	}

	w.resetTimer(path)
}

func (w *Watcher) resetTimer(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}

	delay := w.debounce
	if d, ok := w.dirDebounce[filepath.Dir(path)]; ok {
		delay = d
	}

	w.timers[path] = time.AfterFunc(delay, func() {
		w.mu.Lock()
		delete(w.timers, path)
		w.mu.Unlock()

		// Verify file still exists (it may have been moved/deleted)
		if _, err := os.Stat(path); err != nil {
			return
		}

		w.dispatch(path)
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
