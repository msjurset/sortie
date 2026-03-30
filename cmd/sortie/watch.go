package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/msjurset/sortie/internal/config"
	"github.com/msjurset/sortie/internal/dispatcher"
	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
	"github.com/msjurset/sortie/internal/watcher"
	"github.com/spf13/cobra"
)

var watchFlags struct {
	dryRun    bool
	debounce  time.Duration
	rateLimit time.Duration
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch directories and dispatch files in real time",
	Args:  cobra.NoArgs,
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().BoolVar(&watchFlags.dryRun, "dry-run", false, "log actions without executing")
	watchCmd.Flags().DurationVar(&watchFlags.debounce, "debounce", 500*time.Millisecond, "debounce duration for file events")
	watchCmd.Flags().DurationVar(&watchFlags.rateLimit, "rate-limit", 0, "minimum interval between dispatches (e.g., 500ms, 1s)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	if len(cfg.Directories) == 0 {
		return fmt.Errorf("no directories configured to watch")
	}

	var dirs []string
	for _, d := range cfg.Directories {
		dirs = append(dirs, d.Path)
	}

	logger := appLogger

	store := history.NewStore(cfg.HistoryFile)
	disp := dispatcher.New(store, dispatcher.WithTrashDir(cfg.TrashDir))

	w, err := watcher.New(dirs, watchFlags.debounce, logger)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}

	// Write PID file
	pidPath := filepath.Join(filepath.Dir(cfg.HistoryFile), "sortie.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		logger.Warn("could not write PID file", "err", err)
	}
	defer os.Remove(pidPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Monitor our own binary for changes so the daemon restarts with the
	// new version (launchd KeepAlive will relaunch after exit).
	ctx, stop2 := monitorBinary(ctx, logger)
	defer stop2()

	rl := dispatcher.NewRateLimiter(watchFlags.rateLimit)

	// Start config hot-reload watcher
	cfgReloader := config.NewReloader(cfg, configPath(), logger)
	go cfgReloader.Watch(ctx, dirs)

	fmt.Printf("Watching %d directory(ies)...\n", len(dirs))
	for _, d := range dirs {
		fmt.Printf("  %s\n", d)
	}
	if watchFlags.dryRun {
		fmt.Println("  (dry-run mode)")
	}
	fmt.Println()

	return w.Run(ctx, func(path string) {
		// Use reloader snapshot for each file event
		currentCfg := cfgReloader.Current()

		dir := filepath.Dir(path)
		rules, err := currentCfg.MergedRules(dir)
		if err != nil {
			logger.Error("loading rules", "dir", dir, "err", err)
			return
		}

		fi, err := rule.NewFileInfo(path)
		if err != nil {
			logger.Error("stat file", "path", path, "err", err)
			return
		}

		globalIgnore, localIgnore := currentCfg.EffectiveIgnore(dir)
		if rule.ShouldIgnore(globalIgnore, localIgnore, fi) {
			if verbose {
				logger.Debug("ignored", "file", filepath.Base(path))
			}
			return
		}

		matches := rule.FindMatches(rules, fi)
		if len(matches) == 0 {
			if verbose {
				logger.Debug("no match", "file", filepath.Base(path))
			}
			return
		}

		for _, mr := range matches {
			// Check per-rule cooldown
			if mr.Rule.Cooldown != "" {
				cd, _ := rule.ParseAge(mr.Rule.Cooldown)
				if !rl.AllowRule(mr.Rule.Name, cd) {
					if verbose {
						logger.Debug("cooldown", "rule", mr.Rule.Name, "cooldown", mr.Rule.Cooldown)
					}
					continue
				}
			}

			// Wait for global rate limit
			if err := rl.Wait(ctx); err != nil {
				return
			}

			result, err := disp.Dispatch(fi, *mr.Rule, mr.Captures, watchFlags.dryRun)
			if err != nil {
				logger.Error("dispatch failed", "err", err)
				continue
			}

			rl.Record(mr.Rule.Name)

			logger.Info("dispatched",
				"rule", mr.Rule.Name,
				"action", result.Record.Action,
				"src", filepath.Base(result.Record.Src),
				"dest", result.Record.Dest,
				"dry_run", result.DryRun,
			)
		}
	})
}

// monitorBinary polls the running binary's modification time and cancels the
// context when it changes, allowing launchd's KeepAlive to relaunch with the
// new version.
func monitorBinary(parent context.Context, logger *slog.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	exe, err := os.Executable()
	if err != nil {
		logger.Warn("cannot monitor binary", "err", err)
		return ctx, cancel
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		logger.Warn("cannot resolve binary path", "err", err)
		return ctx, cancel
	}

	info, err := os.Stat(exe)
	if err != nil {
		logger.Warn("cannot stat binary", "err", err)
		return ctx, cancel
	}
	startMod := info.ModTime()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(exe)
				if err != nil {
					continue
				}
				if !info.ModTime().Equal(startMod) {
					logger.Info("binary changed, restarting")
					cancel()
					return
				}
			}
		}
	}()

	return ctx, cancel
}
