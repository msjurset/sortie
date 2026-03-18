package main

import (
	"fmt"

	"github.com/msjurset/sortie/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config
	cfgPath string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:          "sortie",
	Short:        "Intelligent file dispatcher",
	Long:         "Rule-based file routing for directories like ~/Downloads and ~/Desktop.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		return cfg.EnsureDirs()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default ~/.config/sortie/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
