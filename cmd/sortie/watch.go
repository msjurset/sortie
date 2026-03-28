package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/msjurset/sortie/internal/dispatcher"
	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
	"github.com/msjurset/sortie/internal/watcher"
	"github.com/spf13/cobra"
)

var watchFlags struct {
	dryRun   bool
	debounce time.Duration
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

	logger := log.New(os.Stderr, "", log.LstdFlags)

	store := history.NewStore(cfg.HistoryFile)
	disp := dispatcher.New(store, dispatcher.WithTrashDir(cfg.TrashDir))

	w, err := watcher.New(dirs, watchFlags.debounce, logger)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}

	// Write PID file
	pidPath := filepath.Join(filepath.Dir(cfg.HistoryFile), "sortie.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		logger.Printf("warning: could not write PID file: %v", err)
	}
	defer os.Remove(pidPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Monitor our own binary for changes so the daemon restarts with the
	// new version (launchd KeepAlive will relaunch after exit).
	ctx, stop2 := monitorBinary(ctx, logger)
	defer stop2()

	fmt.Printf("Watching %d directory(ies)...\n", len(dirs))
	for _, d := range dirs {
		fmt.Printf("  %s\n", d)
	}
	if watchFlags.dryRun {
		fmt.Println("  (dry-run mode)")
	}
	fmt.Println()

	return w.Run(ctx, func(path string) {
		dir := filepath.Dir(path)
		rules, err := cfg.MergedRules(dir)
		if err != nil {
			logger.Printf("error loading rules for %s: %v", dir, err)
			return
		}

		fi, err := rule.NewFileInfo(path)
		if err != nil {
			logger.Printf("error stat %s: %v", path, err)
			return
		}

		matched := rule.FirstMatch(rules, fi)
		if matched == nil {
			if verbose {
				logger.Printf("no match: %s", filepath.Base(path))
			}
			return
		}

		result, err := disp.Dispatch(fi, *matched, watchFlags.dryRun)
		if err != nil {
			logger.Printf("error: %v", err)
			return
		}

		prefix := ""
		if result.DryRun {
			prefix = "[dry-run] "
		}
		fmt.Printf("%s%s %s -> %s (%s)\n",
			prefix,
			result.Record.Action,
			filepath.Base(result.Record.Src),
			result.Record.Dest,
			matched.Name,
		)
	})
}

// monitorBinary polls the running binary's modification time and cancels the
// context when it changes, allowing launchd's KeepAlive to relaunch with the
// new version.
func monitorBinary(parent context.Context, logger *log.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	exe, err := os.Executable()
	if err != nil {
		logger.Printf("warning: cannot monitor binary: %v", err)
		return ctx, cancel
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		logger.Printf("warning: cannot resolve binary path: %v", err)
		return ctx, cancel
	}

	info, err := os.Stat(exe)
	if err != nil {
		logger.Printf("warning: cannot stat binary: %v", err)
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
					logger.Printf("binary changed, restarting...")
					cancel()
					return
				}
			}
		}
	}()

	return ctx, cancel
}
