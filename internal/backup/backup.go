// Package backup manages snapshot tarballs in ~/.config/sortie/backups/.
//
// One filename pattern lives there: full-state snapshots created by
// `sortie backup snapshot`, named sortie-<YYYY-MM-DDTHHmmss>.tar.gz.
// Each snapshot bundles a curated subset of the user's sortie home:
//
//   - config.yaml   — central config (rules, directories, ignore patterns)
//   - history.json  — operational history (JSON Lines, append-only)
//   - trash/        — files the `delete` action moved to the trash dir
//                     and that haven't been purged yet. Critical: without
//                     this, a restored snapshot can't undo recent deletes.
//
// Excluded:
//
//   - logs/         — daemon stdout/stderr, ephemeral
//   - backups/      — would be recursive
//
// Per-directory `.sortie.yaml` files live inside watched directories
// (e.g. ~/Downloads/.sortie.yaml) and are intentionally outside the
// snapshot. Users back those up alongside the directories that contain
// them.
//
// List, Show, Restore, Diff, Prune, and Snapshot are the public entry
// points. Snapshot is what goback's pre_command invokes; the rest are
// management helpers for browsing what's on disk.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SnapshotPrefix is the leading slot in snapshot filenames.
const SnapshotPrefix = "sortie"

// timestampLayout is the canonical filename timestamp format: ISO 8601-ish
// with capital T and no separators in the time portion.
const timestampLayout = "2006-01-02T150405"

// fileRegex captures (1) prefix, (2) timestamp. Snapshot files only — there
// is no per-file backup variant in sortie.
var fileRegex = regexp.MustCompile(`^(sortie)-(\d{4}-\d{2}-\d{2}T\d{6})\.tar\.gz$`)

// Entry describes one snapshot file.
type Entry struct {
	Timestamp time.Time
	Path      string
	Size      int64
}

// HomeDir returns ~/.config/sortie/.
func HomeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sortie")
}

// DefaultDir returns ~/.config/sortie/backups/.
func DefaultDir() string {
	return filepath.Join(HomeDir(), "backups")
}

// ConfigPath returns the canonical config.yaml path.
func ConfigPath() string {
	return filepath.Join(HomeDir(), "config.yaml")
}

// List returns snapshot entries newest-first. Missing directory is
// treated as empty (not an error) so this works on a fresh install.
func List(dir string) ([]Entry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []Entry
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		m := fileRegex.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		ts, err := time.ParseInLocation(timestampLayout, m[2], time.Local)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		results = append(results, Entry{
			Timestamp: ts,
			Path:      filepath.Join(dir, e.Name()),
			Size:      info.Size(),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results, nil
}

// Find returns the newest entry whose timestamp matches tsPrefix. An empty
// prefix returns the newest entry overall. Returns nil if no entry matches.
func Find(entries []Entry, tsPrefix string) *Entry {
	for i := range entries {
		if tsPrefix == "" {
			return &entries[i]
		}
		if strings.HasPrefix(entries[i].Timestamp.Format(timestampLayout), tsPrefix) {
			return &entries[i]
		}
	}
	return nil
}

// Show returns the tarball's file listing (one path per line). For sortie
// there is only the snapshot tarball variant, so Show always operates on
// a tar.gz.
func Show(backupPath string) ([]byte, error) {
	names, err := readTarballNames(backupPath)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(names, "\n") + "\n"), nil
}

// RestoreConfig extracts config.yaml from a snapshot tarball and writes it
// to targetPath. Before overwriting an existing target, copies the current
// state to backupsDir as a fresh per-file backup so the restore is itself
// reversible. Other items in the snapshot (history.json, trash/) are not
// auto-restored — the user expands those manually with `tar -xzf`, which
// is too destructive for a single command.
func RestoreConfig(backupPath, targetPath, backupsDir string) error {
	data, err := readTarballEntry(backupPath, "config.yaml")
	if err != nil {
		return err
	}

	// Save a copy of the current target before overwriting (if it exists).
	if _, err := os.Stat(targetPath); err == nil {
		ts := time.Now().Format(timestampLayout)
		savedAt := filepath.Join(backupsDir, fmt.Sprintf("config-%s.yaml", ts))
		if err := os.MkdirAll(backupsDir, 0o755); err != nil {
			return err
		}
		if err := copyFile(targetPath, savedAt); err != nil {
			return fmt.Errorf("backing up current config: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0o644)
}

// Diff returns a unified diff between the config.yaml inside the snapshot
// and the live config.yaml at currentPath. Shells out to `diff -u`; exit
// code 1 just means "files differ" and is not an error here.
func Diff(backupPath, currentPath string) ([]byte, error) {
	data, err := readTarballEntry(backupPath, "config.yaml")
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "sortie-diff-*.yaml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	out, err := exec.Command("diff", "-u", tmp.Name(), currentPath).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, nil
		}
		return out, fmt.Errorf("diff: %w", err)
	}
	return out, nil
}

// Prune deletes snapshots exceeding keep AND/OR older than olderThan. Pass
// keep=0 to disable the count rule and olderThan=0 to disable the age rule.
// Returns the paths that were (or would be in dryRun) deleted.
func Prune(dir string, keep int, olderThan time.Duration, dryRun bool) ([]string, error) {
	entries, err := List(dir)
	if err != nil {
		return nil, err
	}

	var toDelete []string
	cutoff := time.Now().Add(-olderThan)
	for i, e := range entries {
		tooOld := olderThan > 0 && e.Timestamp.Before(cutoff)
		beyondKeep := keep > 0 && i >= keep
		if tooOld || beyondKeep {
			toDelete = append(toDelete, e.Path)
		}
	}

	if !dryRun {
		for _, p := range toDelete {
			if err := os.Remove(p); err != nil {
				return toDelete, err
			}
		}
	}
	return toDelete, nil
}

// Snapshot creates a tarball at <outputDir>/sortie-<timestamp>.tar.gz
// containing config.yaml, history.json, and trash/ from sortieHome.
// Missing entries are silently skipped so the snapshot works on a fresh
// install with no history or trash yet.
func Snapshot(sortieHome, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	ts := time.Now().Format(timestampLayout)
	outPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.tar.gz", SnapshotPrefix, ts))

	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, item := range []string{"config.yaml", "history.json", "trash"} {
		full := filepath.Join(sortieHome, item)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		if err := addToTar(tw, sortieHome, item); err != nil {
			return outPath, fmt.Errorf("adding %s: %w", item, err)
		}
	}
	return outPath, nil
}

// addToTar walks relPath (relative to baseDir) and writes its entries.
func addToTar(tw *tar.Writer, baseDir, relPath string) error {
	fullPath := filepath.Join(baseDir, relPath)
	return filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
}

// readTarballNames returns the entries inside a .tar.gz, directories
// stripped of trailing slashes. Used by Show.
func readTarballNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		names = append(names, strings.TrimRight(hdr.Name, "/"))
	}
	return names, nil
}

// readTarballEntry returns the bytes of a single named entry inside a
// .tar.gz. Returns an error if the entry isn't found.
func readTarballEntry(path, want string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("entry %q not found in %s", want, path)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == want {
			return io.ReadAll(tr)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
