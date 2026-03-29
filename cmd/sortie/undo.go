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
		// Undo specific ID (and its chain if part of one)
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
		seen := map[string]bool{} // track chain IDs already collected
		for _, r := range records {
			if r.Undone || r.Error != "" {
				continue
			}
			if r.ChainID != "" && seen[r.ChainID] {
				continue // already counted this chain
			}
			targets = append(targets, r)
			if r.ChainID != "" {
				seen[r.ChainID] = true
			}
			if len(targets) >= undoFlags.last {
				break
			}
		}
	}

	// Expand chain targets: if any target is part of a chain, include all
	// non-undone records in that chain (in reverse order for proper undo).
	targets = expandChains(targets, records)

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

// expandChains replaces chain member targets with all non-undone records
// from their chain, ordered newest-first (for correct reverse undo).
func expandChains(targets, allRecords []history.Record) []history.Record {
	chainIDs := map[string]bool{}
	singleTargets := []history.Record{}

	for _, t := range targets {
		if t.ChainID != "" {
			chainIDs[t.ChainID] = true
		} else {
			singleTargets = append(singleTargets, t)
		}
	}

	if len(chainIDs) == 0 {
		return targets
	}

	// Collect all chain records (allRecords is newest-first, which is the
	// correct undo order — reverse of execution).
	var expanded []history.Record
	for _, r := range allRecords {
		if r.ChainID != "" && chainIDs[r.ChainID] && !r.Undone && r.Error == "" {
			expanded = append(expanded, r)
		}
	}

	return append(expanded, singleTargets...)
}
