package main

import (
	"fmt"
	"path/filepath"

	"github.com/msjurset/sortie/internal/dispatcher"
	"github.com/msjurset/sortie/internal/history"
	"github.com/spf13/cobra"
)

var undoFlags struct {
	last int
}

var undoCmd = &cobra.Command{
	Use:   "undo [id]",
	Short: "Reverse recent dispatch actions",
	Long:  "Undo the most recent action, or a specific action by ID.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUndo,
}

func init() {
	undoCmd.Flags().IntVar(&undoFlags.last, "last", 1, "number of recent actions to undo")
	rootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	store := history.NewStore(cfg.HistoryFile)
	disp := dispatcher.New(store, dispatcher.WithTrashDir(cfg.TrashDir))

	records, err := store.List(0)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No history to undo.")
		return nil
	}

	var targets []history.Record

	if len(args) == 1 {
		// Undo specific ID
		id := args[0]
		for _, r := range records {
			if r.ID == id {
				targets = append(targets, r)
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no record found with ID %q", id)
		}
	} else {
		// Undo last N non-undone, non-errored actions
		for _, r := range records {
			if r.Undone || r.Error != "" {
				continue
			}
			targets = append(targets, r)
			if len(targets) >= undoFlags.last {
				break
			}
		}
	}

	if len(targets) == 0 {
		fmt.Println("No actions available to undo.")
		return nil
	}

	for _, rec := range targets {
		err := disp.Undo(rec)
		if err != nil {
			fmt.Printf("  error undoing %s (%s): %v\n", filepath.Base(rec.Src), rec.ID, err)
			continue
		}

		// Mark as undone in history
		_ = store.Append(history.Record{
			RuleName: rec.RuleName,
			Action:   rec.Action,
			Src:      rec.Src,
			Dest:     rec.Dest,
			Undone:   true,
		})

		fmt.Printf("  undone: %s %s -> %s (%s)\n",
			rec.Action,
			filepath.Base(rec.Src),
			rec.Dest,
			rec.RuleName,
		)
	}

	return nil
}
