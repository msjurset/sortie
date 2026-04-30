package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestList(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"sortie-2026-04-29T100000.tar.gz",
		"sortie-2026-04-28T100000.tar.gz",
		"sortie-2026-04-30T130000.tar.gz",
		"unrelated.txt",
		"sortie.tar.gz",                  // no timestamp — must be skipped
		"runbook-2026-04-29T100000.tar.gz", // wrong prefix — must be skipped
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	// Newest first.
	if !entries[0].Timestamp.Equal(mustTime("2026-04-30T130000")) {
		t.Errorf("newest should be 2026-04-30T130000, got %v", entries[0].Timestamp)
	}
	if !entries[2].Timestamp.Equal(mustTime("2026-04-28T100000")) {
		t.Errorf("oldest should be 2026-04-28T100000, got %v", entries[2].Timestamp)
	}
}

func TestListMissingDir(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing dir should give zero entries, got %d", len(entries))
	}
}

func TestFind(t *testing.T) {
	entries := []Entry{
		{Timestamp: mustTime("2026-04-30T130000")},
		{Timestamp: mustTime("2026-04-29T080000")},
		{Timestamp: mustTime("2026-04-28T100000")},
	}

	got := Find(entries, "")
	if got == nil || !got.Timestamp.Equal(mustTime("2026-04-30T130000")) {
		t.Errorf("empty prefix should return newest, got %+v", got)
	}
	got = Find(entries, "2026-04-29")
	if got == nil || !got.Timestamp.Equal(mustTime("2026-04-29T080000")) {
		t.Errorf("prefix match: got %+v", got)
	}
	got = Find(entries, "2026-04-29T08")
	if got == nil || !got.Timestamp.Equal(mustTime("2026-04-29T080000")) {
		t.Errorf("hour-prefix match: got %+v", got)
	}
	got = Find(entries, "2026-01")
	if got != nil {
		t.Errorf("no match expected, got %+v", got)
	}
}

func TestPrune_Keep(t *testing.T) {
	dir := t.TempDir()
	for _, ts := range []string{
		"2026-04-30T100000",
		"2026-04-29T100000",
		"2026-04-28T100000",
		"2026-04-27T100000",
		"2026-04-26T100000",
	} {
		if err := os.WriteFile(filepath.Join(dir, "sortie-"+ts+".tar.gz"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := Prune(dir, 2, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 3 {
		t.Fatalf("want 3 deleted, got %d", len(deleted))
	}
	remaining, _ := List(dir)
	if len(remaining) != 2 {
		t.Fatalf("want 2 remaining, got %d", len(remaining))
	}
	if !remaining[0].Timestamp.Equal(mustTime("2026-04-30T100000")) ||
		!remaining[1].Timestamp.Equal(mustTime("2026-04-29T100000")) {
		t.Errorf("wrong files remained: %+v", remaining)
	}
}

func TestPrune_OlderThan(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(age time.Duration) {
		ts := now.Add(-age).Format(timestampLayout)
		f := filepath.Join(dir, "sortie-"+ts+".tar.gz")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(5 * time.Hour)
	mk(30 * 24 * time.Hour)
	mk(60 * 24 * time.Hour)

	deleted, err := Prune(dir, 0, 7*24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 {
		t.Fatalf("want 2 deleted (older than 7d), got %d", len(deleted))
	}
}

func TestPrune_DryRun(t *testing.T) {
	dir := t.TempDir()
	for _, ts := range []string{"2026-04-30T100000", "2026-04-29T100000", "2026-04-28T100000"} {
		if err := os.WriteFile(filepath.Join(dir, "sortie-"+ts+".tar.gz"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := Prune(dir, 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(deleted))
	}
	remaining, _ := List(dir)
	if len(remaining) != 3 {
		t.Fatalf("dry-run should not delete; got %d remaining", len(remaining))
	}
}

func TestSnapshot(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "history.json"), []byte(`{"id":"x"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// trash/, logs/, and backups/ are all populated to confirm they're
	// excluded from the tarball (whitelist behavior — Snapshot only adds
	// the items it knows about).
	for _, sub := range []string{"trash", "logs", "backups"} {
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, sub, "junk.txt"), []byte("noise"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	outDir := t.TempDir()
	tarPath, err := Snapshot(home, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(tarPath, ".tar.gz") {
		t.Errorf("snapshot path should be .tar.gz, got %s", tarPath)
	}
	if !strings.Contains(filepath.Base(tarPath), "sortie-") {
		t.Errorf("snapshot path should start with 'sortie-', got %s", tarPath)
	}

	names := readTarballEntries(t, tarPath)
	want := map[string]bool{
		"config.yaml":  false,
		"history.json": false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("missing %q in snapshot tarball", n)
		}
	}
	for _, n := range names {
		for _, excluded := range []string{"trash", "logs", "backups"} {
			if n == excluded || strings.HasPrefix(n, excluded+"/") {
				t.Errorf("%s/ should be excluded from snapshot, got %q", excluded, n)
			}
		}
	}
}

func TestSnapshot_FreshInstall(t *testing.T) {
	// No config.yaml, no history.json, no trash/ — should still produce a
	// (mostly empty) tarball without error.
	home := t.TempDir()
	outDir := t.TempDir()
	tarPath, err := Snapshot(home, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tarPath); err != nil {
		t.Errorf("expected tarball at %s, got %v", tarPath, err)
	}
}

func TestRestoreConfig(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("rules: [a]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(home, "backups")
	tarPath, err := Snapshot(home, outDir)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the live config and restore from the snapshot.
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("rules: [b]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RestoreConfig(tarPath, filepath.Join(home, "config.yaml"), outDir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "rules: [a]\n" {
		t.Errorf("restored content = %q, want %q", string(got), "rules: [a]\n")
	}

	// The pre-restore state should have been preserved.
	saved, _ := filepath.Glob(filepath.Join(outDir, "config-*.yaml"))
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved pre-restore config, got %d", len(saved))
	}
	preRestore, _ := os.ReadFile(saved[0])
	if string(preRestore) != "rules: [b]\n" {
		t.Errorf("pre-restore copy = %q, want %q", string(preRestore), "rules: [b]\n")
	}
}

func readTarballEntries(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
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
			t.Fatal(err)
		}
		names = append(names, strings.TrimRight(hdr.Name, "/"))
	}
	sort.Strings(names)
	return names
}

func mustTime(s string) time.Time {
	t, err := time.ParseInLocation(timestampLayout, s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}
