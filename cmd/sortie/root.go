package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/msjurset/sortie/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg       *config.Config
	cfgPath   string
	verbose   bool
	logFormat string
	appLogger *slog.Logger
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

		// Initialize structured logger
		format := logFormat
		if format == "" {
			format = cfg.LogFormat
		}
		if format == "" {
			format = "text"
		}

		var handler slog.Handler
		switch format {
		case "json":
			handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
		default:
			handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
		}
		appLogger = slog.New(handler)

		return cfg.EnsureDirs()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default ~/.config/sortie/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "log output format: text (default) or json")
}
