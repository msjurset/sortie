package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/msjurset/sortie/internal/backup"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage sortie state snapshots",
	Long: `Manage snapshot tarballs in ~/.config/sortie/backups/.

Each snapshot bundles the data sortie uniquely owns — central config and
dispatch history — into a single tar.gz so your rules and audit log are
portable across machines and recoverable after accidents.

Snapshot contents:
  - config.yaml   — central config (rules, directories, ignore patterns)
  - history.json  — dispatch history (JSON Lines)

Excluded:
  - trash/        — transient state; recover deleted files via 'sortie
                    undo' or your filesystem-level backup
  - logs/         — daemon stdout/stderr, ephemeral
  - backups/      — would be recursive
  - per-directory .sortie.yaml files (those live inside watched
    directories like ~/Downloads/.sortie.yaml — back them up alongside
    their parent directories)

The 'snapshot' command is goback-friendly: paired with a goback 'local'
job that picks up the tarball, you get scheduled off-app backups.`,
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List snapshot tarballs, newest first",
	Args:  cobra.NoArgs,
	RunE:  runBackupList,
}

var backupShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the file listing of a snapshot tarball",
	Args:  cobra.NoArgs,
	RunE:  runBackupShow,
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore config.yaml from a snapshot (other items require manual tar -xzf)",
	Long: `Extract config.yaml from a snapshot tarball and overwrite the live
~/.config/sortie/config.yaml. Before overwriting, the current config.yaml
is saved alongside the snapshots as config-<ISO timestamp>.yaml so the
restore is itself reversible.

Only config.yaml is auto-restored. To recover history.json or trash/,
expand the tarball manually:

    tar -xzf ~/.config/sortie/backups/sortie-<ts>.tar.gz -C ~/.config/sortie/

That's not done automatically because it would silently rewrite many
files at once.`,
	Args: cobra.NoArgs,
	RunE: runBackupRestore,
}

var backupDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff config.yaml inside a snapshot against the current config.yaml",
	Args:  cobra.NoArgs,
	RunE:  runBackupDiff,
}

var backupPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete old snapshots by count and/or age",
	Args:  cobra.NoArgs,
	RunE:  runBackupPrune,
}

var backupSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Create a tarball at ~/.config/sortie/backups/sortie-<ts>.tar.gz",
	Args:  cobra.NoArgs,
	RunE:  runBackupSnapshot,
}

func init() {
	backupShowCmd.Flags().String("at", "", "Match a specific timestamp prefix (e.g. 2026-04-30 or 2026-04-30T08)")
	backupRestoreCmd.Flags().String("at", "", "Match a specific timestamp prefix")
	backupDiffCmd.Flags().String("at", "", "Match a specific timestamp prefix")
	backupPruneCmd.Flags().Int("keep", 10, "Max snapshots to keep. 0 disables this rule.")
	backupPruneCmd.Flags().String("older-than", "", "Delete snapshots older than this duration (e.g. 7d, 30d, 24h)")
	backupPruneCmd.Flags().Bool("dry-run", false, "Print what would be deleted without deleting")

	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupShowCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupDiffCmd)
	backupCmd.AddCommand(backupPruneCmd)
	backupCmd.AddCommand(backupSnapshotCmd)

	rootCmd.AddCommand(backupCmd)
}

func runBackupList(cmd *cobra.Command, args []string) error {
	entries, err := backup.List(backup.DefaultDir())
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tSIZE\tPATH")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			e.Timestamp.Local().Format("2006-01-02 15:04:05"),
			humanBytes(e.Size),
			e.Path,
		)
	}
	return w.Flush()
}

func runBackupShow(cmd *cobra.Command, args []string) error {
	entry, err := resolveSnapshot(cmd)
	if err != nil {
		return err
	}
	out, err := backup.Show(entry.Path)
	if err != nil {
		return err
	}
	os.Stdout.Write(out)
	return nil
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	entry, err := resolveSnapshot(cmd)
	if err != nil {
		return err
	}
	target := backup.ConfigPath()
	if err := backup.RestoreConfig(entry.Path, target, backup.DefaultDir()); err != nil {
		return err
	}
	fmt.Printf("✓ Restored %s from %s\n", target, filepath.Base(entry.Path))
	fmt.Printf("  Pre-restore copy saved alongside snapshots in %s\n", backup.DefaultDir())
	fmt.Printf("  To restore history.json or trash/, expand manually:\n")
	fmt.Printf("    tar -xzf %s -C %s\n", entry.Path, backup.HomeDir())
	return nil
}

func runBackupDiff(cmd *cobra.Command, args []string) error {
	entry, err := resolveSnapshot(cmd)
	if err != nil {
		return err
	}
	out, err := backup.Diff(entry.Path, backup.ConfigPath())
	if err != nil {
		return err
	}
	if len(out) == 0 {
		fmt.Printf("No differences between snapshot %s and current config.\n", filepath.Base(entry.Path))
		return nil
	}
	os.Stdout.Write(out)
	return nil
}

func runBackupPrune(cmd *cobra.Command, args []string) error {
	keep, _ := cmd.Flags().GetInt("keep")
	olderStr, _ := cmd.Flags().GetString("older-than")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	var olderThan time.Duration
	if olderStr != "" {
		d, err := parseBackupDayDuration(olderStr)
		if err != nil {
			return fmt.Errorf("invalid --older-than %q: %w", olderStr, err)
		}
		olderThan = d
	}
	if keep == 0 && olderThan == 0 {
		return fmt.Errorf("specify at least one of --keep N or --older-than DURATION (use --keep 0 + --older-than to disable count rule)")
	}

	deleted, err := backup.Prune(backup.DefaultDir(), keep, olderThan, dryRun)
	if err != nil {
		return err
	}
	if len(deleted) == 0 {
		fmt.Println("No snapshots to prune.")
		return nil
	}
	verb := "Deleted"
	if dryRun {
		verb = "Would delete"
	}
	fmt.Printf("%s %d snapshot(s):\n", verb, len(deleted))
	for _, p := range deleted {
		fmt.Printf("  %s\n", p)
	}
	return nil
}

func runBackupSnapshot(cmd *cobra.Command, args []string) error {
	out, err := backup.Snapshot(backup.HomeDir(), backup.DefaultDir())
	if err != nil {
		return err
	}
	info, _ := os.Stat(out)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Printf("✓ Created %s (%s)\n", out, humanBytes(size))
	return nil
}

// resolveSnapshot returns the snapshot entry matching --at (or the newest
// if --at is empty). Returns a usable error message when nothing matches.
func resolveSnapshot(cmd *cobra.Command) (*backup.Entry, error) {
	tsPrefix, _ := cmd.Flags().GetString("at")
	entries, err := backup.List(backup.DefaultDir())
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no snapshots found in %s — create one with `sortie backup snapshot`", backup.DefaultDir())
	}
	hit := backup.Find(entries, tsPrefix)
	if hit == nil {
		return nil, fmt.Errorf("no snapshot matching --at %q (run `sortie backup list` to see what's available)", tsPrefix)
	}
	return hit, nil
}

// parseBackupDayDuration extends time.ParseDuration to accept Nd for days.
var backupDayDurRe = regexp.MustCompile(`^(\d+)d$`)

func parseBackupDayDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if m := backupDayDurRe.FindStringSubmatch(s); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// humanBytes renders a byte count as a short string.
func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1fM", float64(n)/1024/1024)
	default:
		return fmt.Sprintf("%.1fG", float64(n)/1024/1024/1024)
	}
}
