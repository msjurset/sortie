package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/msjurset/sortie/internal/history"
	"github.com/spf13/cobra"
)

var statusFlags struct {
	watch bool
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show watcher status",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusFlags.watch, "watch", false, "live tail of dispatch activity")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	pidPath := filepath.Join(filepath.Dir(cfg.HistoryFile), "sortie.pid")

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Status: not running")
			return nil
		}
		return fmt.Errorf("reading PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("Status: not running (invalid PID file)")
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Status: not running")
		return nil
	}

	// Signal 0 checks if process exists without sending a real signal
	if err := process.Signal(syscall.Signal(0)); err != nil {
		fmt.Printf("Status: not running (stale PID %d)\n", pid)
		os.Remove(pidPath)
		return nil
	}

	fmt.Printf("Status: running (PID %d)\n", pid)

	fmt.Printf("\nWatched directories:\n")
	for _, d := range cfg.Directories {
		fmt.Printf("  %s\n", d.Path)
	}

	ruleCount := len(cfg.Rules)
	fmt.Printf("\nGlobal rules: %d\n", ruleCount)

	if statusFlags.watch {
		fmt.Println("\nWatching for activity...")
		return tailHistory(cfg.HistoryFile)
	}

	return nil
}

// tailHistory seeks to the end of the history file and polls for new records,
// printing each one as a formatted line.
func tailHistory(path string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet — wait for it
			f, err = waitForFile(ctx, path)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("opening history: %w", err)
		}
	}
	defer f.Close()

	// Seek to end
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}

	scanner := bufio.NewScanner(f)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for scanner.Scan() {
				var rec history.Record
				if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
					continue
				}
				if rec.Undone {
					continue
				}
				printRecord(rec)
			}
		}
	}
}

func printRecord(rec history.Record) {
	ts := rec.Timestamp.Local().Format("15:04:05")
	src := filepath.Base(rec.Src)

	if rec.Error != "" {
		fmt.Printf("[%s] ERROR %s %s: %s (%s)\n", ts, rec.Action, src, rec.Error, rec.RuleName)
		return
	}

	if rec.Dest != "" {
		fmt.Printf("[%s] %s %s -> %s (%s)\n", ts, rec.Action, src, rec.Dest, rec.RuleName)
	} else {
		fmt.Printf("[%s] %s %s (%s)\n", ts, rec.Action, src, rec.RuleName)
	}
}

func waitForFile(ctx context.Context, path string) (*os.File, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			f, err := os.Open(path)
			if err == nil {
				return f, nil
			}
		}
	}
}
