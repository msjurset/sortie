package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/msjurset/sortie/internal/history"
	"github.com/spf13/cobra"
)

var historyFlags struct {
	limit int
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show action history",
	Args:  cobra.NoArgs,
	RunE:  runHistory,
}

func init() {
	historyCmd.Flags().IntVarP(&historyFlags.limit, "limit", "n", 20, "max records to show")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	store := history.NewStore(cfg.HistoryFile)
	records, err := store.List(historyFlags.limit)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No history found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tACTION\tFILE\tDEST\tRULE")
	for _, r := range records {
		status := r.Action
		if r.Error != "" {
			status += " (err)"
		}
		if r.Undone {
			status += " (undone)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Timestamp.Local().Format("2006-01-02 15:04"),
			status,
			filepath.Base(r.Src),
			r.Dest,
			r.RuleName,
		)
	}
	return w.Flush()
}
