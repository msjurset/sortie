package dispatcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
)

// --- Open ---

func TestDispatchOpenDefault(t *testing.T) {
	if _, err := exec.LookPath("open"); err != nil {
		t.Skip("open not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello")

	r := rule.Rule{
		Name:   "test-open",
		Action: rule.Action{Type: rule.ActionOpen},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "open" {
		t.Errorf("action = %q, want %q", result.Record.Action, "open")
	}
}

func TestDispatchOpenWithApp(t *testing.T) {
	if _, err := exec.LookPath("open"); err != nil {
		t.Skip("open not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello")

	r := rule.Rule{
		Name:   "test-open-app",
		Action: rule.Action{Type: rule.ActionOpen, App: "TextEdit"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "open" {
		t.Errorf("action = %q, want %q", result.Record.Action, "open")
	}
}

func TestUndoOpenNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(history.Record{Action: "open", Src: "/file.txt"})
	if err == nil {
		t.Fatal("expected error from undo open")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

// --- Deduplicate ---

func TestDispatchDeduplicateNoDuplicate(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "report.pdf", "unique content")

	destPath := filepath.Join(destDir, "report.pdf")
	r := rule.Rule{
		Name:   "test-dedup",
		Action: rule.Action{Type: rule.ActionDeduplicate, Dest: destPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Should have moved the file
	if !strings.HasPrefix(result.Record.Dest, "moved:") {
		t.Errorf("dest = %q, want 'moved:...'", result.Record.Dest)
	}

	// Source should be gone
	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source should be removed after move")
	}

	// Dest should exist
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(data) != "unique content" {
		t.Errorf("content = %q, want %q", string(data), "unique content")
	}
}

func TestDispatchDeduplicateSkip(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create identical file at dest
	content := "identical content"
	fi := testFileInfo(t, srcDir, "report.pdf", content)
	destPath := filepath.Join(destDir, "report.pdf")
	if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := rule.Rule{
		Name:   "test-dedup-skip",
		Action: rule.Action{Type: rule.ActionDeduplicate, Dest: destPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if !strings.HasPrefix(result.Record.Dest, "skip:") {
		t.Errorf("dest = %q, want 'skip:...'", result.Record.Dest)
	}

	// Source should still exist (skipped)
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source should still exist when duplicate is skipped")
	}
}

func TestDispatchDeduplicateDelete(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	content := "identical content"
	fi := testFileInfo(t, srcDir, "report.pdf", content)
	destPath := filepath.Join(destDir, "report.pdf")
	if err := os.WriteFile(destPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := rule.Rule{
		Name:   "test-dedup-delete",
		Action: rule.Action{Type: rule.ActionDeduplicate, Dest: destPath, OnDuplicate: "delete"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if !strings.HasPrefix(result.Record.Dest, "delete:") {
		t.Errorf("dest = %q, want 'delete:...'", result.Record.Dest)
	}

	// Source should be removed (deleted as duplicate)
	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source should be removed when on_duplicate=delete")
	}
}

func TestDispatchDeduplicateDifferentContent(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "report.pdf", "new version")
	destPath := filepath.Join(destDir, "report.pdf")
	if err := os.WriteFile(destPath, []byte("old version"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := rule.Rule{
		Name:   "test-dedup-different",
		Action: rule.Action{Type: rule.ActionDeduplicate, Dest: destPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Different content — should move (overwrite)
	if !strings.HasPrefix(result.Record.Dest, "moved:") {
		t.Errorf("dest = %q, want 'moved:...'", result.Record.Dest)
	}
}

func TestDispatchDeduplicateMissingDest(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "file.txt", "data")

	r := rule.Rule{
		Name:   "test-dedup-no-dest",
		Action: rule.Action{Type: rule.ActionDeduplicate},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing dest")
	}
	if !strings.Contains(err.Error(), "requires dest") {
		t.Errorf("error = %q, want 'requires dest'", err.Error())
	}
}

func TestUndoDeduplicateMoved(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "file.txt", "content")
	destPath := filepath.Join(destDir, "file.txt")

	r := rule.Rule{
		Name:   "test-dedup",
		Action: rule.Action{Type: rule.ActionDeduplicate, Dest: destPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	// Source should be restored
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		t.Fatalf("source should be restored: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}
}

func TestUndoDeduplicateSkip(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	// Skip means nothing happened — undo should succeed silently
	err := disp.Undo(history.Record{Action: "deduplicate", Src: "/src", Dest: "skip:/dest"})
	if err != nil {
		t.Fatalf("Undo() error: %v", err)
	}
}

func TestUndoDeduplicateDelete(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(history.Record{Action: "deduplicate", Src: "/src", Dest: "delete:/dest"})
	if err == nil {
		t.Fatal("expected error for undo of delete dedup")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

// --- Unquarantine ---

func TestDispatchUnquarantine(t *testing.T) {
	if _, err := exec.LookPath("xattr"); err != nil {
		t.Skip("xattr not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "app.dmg", "fake disk image")

	// Set quarantine attribute
	exec.Command("xattr", "-w", "com.apple.quarantine", "0081;deadbeef;Safari;", fi.Path).Run()

	r := rule.Rule{
		Name:   "test-unquarantine",
		Action: rule.Action{Type: rule.ActionUnquarantine},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "unquarantine" {
		t.Errorf("action = %q, want %q", result.Record.Action, "unquarantine")
	}

	// Verify quarantine attribute is gone
	out, _ := exec.Command("xattr", "-l", fi.Path).CombinedOutput()
	if strings.Contains(string(out), "com.apple.quarantine") {
		t.Error("quarantine xattr should be removed")
	}
}

func TestDispatchUnquarantineNoAttr(t *testing.T) {
	if _, err := exec.LookPath("xattr"); err != nil {
		t.Skip("xattr not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "clean.txt", "no quarantine")

	r := rule.Rule{
		Name:   "test-unquarantine-noop",
		Action: rule.Action{Type: rule.ActionUnquarantine},
	}

	// Should succeed even if no quarantine attribute present
	_, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}
}

func TestUndoUnquarantineNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(history.Record{Action: "unquarantine", Src: "/file.dmg"})
	if err == nil {
		t.Fatal("expected error from undo unquarantine")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

// --- hashFile ---

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile() error: %v", err)
	}

	// Same content should produce same hash
	path2 := filepath.Join(dir, "test2.bin")
	if err := os.WriteFile(path2, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	h2, err := hashFile(path2)
	if err != nil {
		t.Fatalf("hashFile() error: %v", err)
	}

	if h1 != h2 {
		t.Errorf("same content should produce same hash: %s != %s", h1, h2)
	}

	// Different content should produce different hash
	path3 := filepath.Join(dir, "test3.bin")
	if err := os.WriteFile(path3, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	h3, err := hashFile(path3)
	if err != nil {
		t.Fatalf("hashFile() error: %v", err)
	}

	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
}
