package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration",
	Args:  cobra.NoArgs,
	RunE:  runConfigShow,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter config file",
	Args:  cobra.NoArgs,
	RunE:  runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	Args:  cobra.NoArgs,
	RunE:  runConfigPath,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Printf("Config file: %s\n", configPath())
	fmt.Printf("Log dir:     %s\n", cfg.LogDir)
	fmt.Printf("History:     %s\n", cfg.HistoryFile)
	fmt.Printf("Trash:       %s\n", cfg.TrashDir)

	fmt.Printf("\nDirectories: %d\n", len(cfg.Directories))
	for _, d := range cfg.Directories {
		r := ""
		if d.Recursive {
			r = " (recursive)"
		}
		fmt.Printf("  %s%s\n", d.Path, r)
	}

	fmt.Printf("\nGlobal rules: %d\n", len(cfg.Rules))
	for _, rule := range cfg.Rules {
		fmt.Printf("  %s (%s)\n", rule.Name, rule.Action.Type)
	}

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Println(configPath())
	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	path := configPath()

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	starter := `# sortie configuration
# See: sortie --help

# Directories to watch (used by 'sortie watch' and 'sortie scan')
directories:
  - path: ~/Downloads
    recursive: false
  # - path: ~/Desktop
  #   recursive: false

# Global rules — evaluated in order, first match wins.
# Per-directory rules (.sortie.yaml) take precedence over these.
rules:
  # - name: images-to-photos
  #   match:
  #     extensions: [.jpg, .jpeg, .png, .heic, .gif, .webp]
  #   action:
  #     type: move
  #     dest: ~/Pictures/Sorted/{{.Year}}/{{.Month}}

  # - name: old-downloads
  #   match:
  #     min_age: 90d
  #   action:
  #     type: delete

  # - name: large-files
  #   match:
  #     min_size: 500MB
  #   action:
  #     type: move
  #     dest: ~/LargeFiles
`

	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}

func configPath() string {
	if cfgPath != "" {
		return cfgPath
	}
	return defaultConfigPath()
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sortie", "config.yaml")
}
