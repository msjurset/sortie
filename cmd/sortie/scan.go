package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/msjurset/sortie/internal/dispatcher"
	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
	"github.com/spf13/cobra"
)

var scanFlags struct {
	dryRun    bool
	rateLimit time.Duration
}

var scanCmd = &cobra.Command{
	Use:   "scan [directory...]",
	Short: "Scan directories and apply rules",
	Long:  "Scan one or more directories (or all configured directories) and dispatch files according to rules.",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().BoolVar(&scanFlags.dryRun, "dry-run", false, "preview actions without executing")
	scanCmd.Flags().DurationVar(&scanFlags.rateLimit, "rate-limit", 0, "minimum interval between dispatches (e.g., 500ms, 1s)")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	dirs := args
	if len(dirs) == 0 {
		for _, d := range cfg.Directories {
			dirs = append(dirs, d.Path)
		}
	}

	if len(dirs) == 0 {
		return fmt.Errorf("no directories specified and none configured")
	}

	store := history.NewStore(cfg.HistoryFile)
	disp := dispatcher.New(store, dispatcher.WithTrashDir(cfg.TrashDir))
	rl := dispatcher.NewRateLimiter(scanFlags.rateLimit)

	var totalActions int

	for _, dir := range dirs {
		dir = expandHome(dir)
		n, err := scanDir(disp, rl, dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error scanning %s: %v\n", dir, err)
			continue
		}
		totalActions += n
	}

	if totalActions == 0 {
		fmt.Println("No matching files found.")
	} else if scanFlags.dryRun {
		fmt.Printf("\n%d action(s) would be taken (dry run).\n", totalActions)
	} else {
		fmt.Printf("\n%d file(s) dispatched.\n", totalActions)
	}

	return nil
}

func scanDir(disp *dispatcher.Dispatcher, rl *dispatcher.RateLimiter, dir string) (int, error) {
	rules, err := cfg.MergedRules(dir)
	if err != nil {
		return 0, fmt.Errorf("loading rules for %s: %w", dir, err)
	}

	if len(rules) == 0 {
		if verbose {
			fmt.Printf("  %s: no rules configured, skipping\n", dir)
		}
		return 0, nil
	}

	globalIgnore, localIgnore := cfg.EffectiveIgnore(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var count int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == ".sortie.yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		fi, err := rule.NewFileInfo(path)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "  skip %s: %v\n", path, err)
			}
			continue
		}

		if rule.ShouldIgnore(globalIgnore, localIgnore, fi) {
			if verbose {
				fmt.Printf("  %s: ignored\n", fi.Info.Name())
			}
			continue
		}

		matches := rule.FindMatches(rules, fi)
		if len(matches) == 0 {
			if verbose {
				fmt.Printf("  %s: no match\n", fi.Info.Name())
			}
			continue
		}

		for _, matched := range matches {
			// Check per-rule cooldown
			if matched.Cooldown != "" {
				cd, _ := rule.ParseAge(matched.Cooldown)
				if !rl.AllowRule(matched.Name, cd) {
					if verbose {
						fmt.Printf("  %s: cooldown (%s)\n", fi.Info.Name(), matched.Cooldown)
					}
					continue
				}
			}

			// Wait for global rate limit
			if err := rl.Wait(context.Background()); err != nil {
				return count, err
			}

			result, err := disp.Dispatch(fi, *matched, scanFlags.dryRun)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error: %v\n", err)
				continue
			}

			prefix := " "
			if result.DryRun {
				prefix = "  [dry-run]"
			}
			fmt.Printf("%s %s %s -> %s (%s)\n",
				prefix,
				result.Record.Action,
				filepath.Base(result.Record.Src),
				result.Record.Dest,
				matched.Name,
			)
			rl.Record(matched.Name)
			count++
		}
	}

	return count, nil
}

func expandHome(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
