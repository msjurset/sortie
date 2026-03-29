package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msjurset/sortie/internal/config"
	"github.com/msjurset/sortie/internal/rule"
	"github.com/spf13/cobra"
)

var validateFlags struct {
	global bool
}

var validateCmd = &cobra.Command{
	Use:   "validate [directory...]",
	Short: "Validate rules for errors and potential problems",
	Long: `Check rules for configuration errors, missing required fields,
incompatible action chains, and potential infinite loop risks where
destinations overlap with watched directories.

With no arguments, validates all rules (global and per-directory).
With directory arguments, validates only the per-directory rules for
those directories. Use --global to include global rules in the check.

Examples:
  sortie validate                  # validate all rules
  sortie validate .                # validate .sortie.yaml in current directory
  sortie validate ~/Downloads      # validate .sortie.yaml in ~/Downloads
  sortie validate . --global       # include global rules in the check

Exit codes:
  0  No errors found (warnings may still be present)
  1  One or more errors found`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&validateFlags.global, "global", false, "include global rules when validating specific directories")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	var allRules []rule.Rule
	var watchedDirs []rule.WatchedDir
	var sources []string

	for _, d := range cfg.Directories {
		watchedDirs = append(watchedDirs, rule.WatchedDir{Path: d.Path, Recursive: d.Recursive})
	}

	if len(args) > 0 {
		for _, dir := range args {
			dir = expandHome(dir)
			if abs, err := filepath.Abs(dir); err == nil {
				dir = abs
			}
			sources = append(sources, dir)

			dc, err := config.LoadDirConfig(dir)
			if err != nil {
				return fmt.Errorf("loading %s: %w", dir, err)
			}
			if dc != nil {
				allRules = append(allRules, dc.Rules...)
			}
		}
		if validateFlags.global {
			allRules = append(allRules, cfg.Rules...)
		}
	} else {
		// Validate everything
		for _, d := range cfg.Directories {
			sources = append(sources, d.Path)

			dc, err := config.LoadDirConfig(d.Path)
			if err != nil {
				return fmt.Errorf("loading rules for %s: %w", d.Path, err)
			}
			if dc != nil {
				allRules = append(allRules, dc.Rules...)
			}
		}
		allRules = append(allRules, cfg.Rules...)
	}

	// Deduplicate by rule name
	seen := map[string]bool{}
	var unique []rule.Rule
	for _, r := range allRules {
		if !seen[r.Name] {
			seen[r.Name] = true
			unique = append(unique, r)
		}
	}

	if len(unique) == 0 {
		fmt.Printf("No rules found for %s\n", strings.Join(sources, ", "))
		return nil
	}

	findings := rule.ValidateRules(unique, watchedDirs)

	if len(findings) == 0 {
		fmt.Printf("All %d rule(s) valid.\n", len(unique))
		return nil
	}

	hasError := false
	for _, f := range findings {
		if f.Severity == rule.SeverityError {
			hasError = true
		}
		fmt.Println(f)
	}

	fmt.Printf("\n%d finding(s) in %d rule(s)\n", len(findings), len(unique))

	if hasError {
		os.Exit(1)
	}
	return nil
}
