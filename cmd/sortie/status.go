package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show watcher status",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
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

	return nil
}
