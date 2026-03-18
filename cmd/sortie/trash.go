package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var trashCmd = &cobra.Command{
	Use:   "trash",
	Short: "Manage the trash directory",
	Args:  cobra.NoArgs,
	RunE:  runTrashList,
}

var trashPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Permanently delete all files in trash",
	Args:  cobra.NoArgs,
	RunE:  runTrashPurge,
}

func init() {
	trashCmd.AddCommand(trashPurgeCmd)
	rootCmd.AddCommand(trashCmd)
}

func runTrashList(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(cfg.TrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Trash is empty.")
			return nil
		}
		return fmt.Errorf("reading trash: %w", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Printf("  %s  (%s)\n", e.Name(), humanSize(info.Size()))
		count++
	}

	if count == 0 {
		fmt.Println("Trash is empty.")
	} else {
		fmt.Printf("\n%d file(s) in trash (%s)\n", count, cfg.TrashDir)
	}

	return nil
}

func runTrashPurge(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(cfg.TrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Trash is already empty.")
			return nil
		}
		return fmt.Errorf("reading trash: %w", err)
	}

	var count int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := cfg.TrashDir + "/" + e.Name()
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing %s: %v\n", e.Name(), err)
			continue
		}
		count++
	}

	fmt.Printf("Purged %d file(s) from trash.\n", count)
	return nil
}

func humanSize(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
