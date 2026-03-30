package dispatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
)

func testFileInfo(t *testing.T, dir, name string, content string) rule.FileInfo {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := rule.NewFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func newTestDispatcher(t *testing.T) (*Dispatcher, string) {
	t.Helper()
	histDir := t.TempDir()
	trashDir := t.TempDir()
	store := history.NewStore(filepath.Join(histDir, "history.json"))
	return New(store, WithTrashDir(trashDir)), trashDir
}

func TestDispatchMove(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello world")

	r := rule.Rule{
		Name:   "test-move",
		Action: rule.Action{Type: rule.ActionMove, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "move" {
		t.Errorf("action = %q, want %q", result.Record.Action, "move")
	}

	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source file should be removed after move")
	}

	destPath := filepath.Join(destDir, "test.txt")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("dest content = %q, want %q", string(data), "hello world")
	}
}

func TestDispatchCopy(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello world")

	r := rule.Rule{
		Name:   "test-copy",
		Action: rule.Action{Type: rule.ActionCopy, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "copy" {
		t.Errorf("action = %q, want %q", result.Record.Action, "copy")
	}

	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source file should still exist after copy")
	}

	destPath := filepath.Join(destDir, "test.txt")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("dest content = %q, want %q", string(data), "hello world")
	}
}

func TestDispatchDryRun(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello world")

	r := rule.Rule{
		Name:   "test-dry",
		Action: rule.Action{Type: rule.ActionMove, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, true)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}

	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source file should still exist in dry-run mode")
	}
}

func TestDispatchDelete(t *testing.T) {
	srcDir := t.TempDir()
	disp, trashDir := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "junk.txt", "delete me")

	r := rule.Rule{
		Name:   "test-delete",
		Action: rule.Action{Type: rule.ActionDelete},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "delete" {
		t.Errorf("action = %q, want %q", result.Record.Action, "delete")
	}

	// Source should be gone
	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source file should be removed after delete")
	}

	// Should be in trash
	trashFile := filepath.Join(trashDir, "junk.txt")
	data, err := os.ReadFile(trashFile)
	if err != nil {
		t.Fatalf("reading trash file: %v", err)
	}
	if string(data) != "delete me" {
		t.Errorf("trash content = %q, want %q", string(data), "delete me")
	}
}

func TestDispatchCompress(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.log", "log content here")

	r := rule.Rule{
		Name:   "test-compress",
		Action: rule.Action{Type: rule.ActionCompress, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "compress" {
		t.Errorf("action = %q, want %q", result.Record.Action, "compress")
	}

	// Source should be removed
	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source file should be removed after compress")
	}

	// Compressed file should exist
	if !strings.HasSuffix(result.Record.Dest, ".gz") {
		t.Errorf("dest should end in .gz, got %q", result.Record.Dest)
	}
	if _, err := os.Stat(result.Record.Dest); err != nil {
		t.Errorf("compressed file should exist: %v", err)
	}
}

func TestDispatchRename(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "old.txt", "content")

	destPath := filepath.Join(srcDir, "new.txt")
	r := rule.Rule{
		Name:   "test-rename",
		Action: rule.Action{Type: rule.ActionRename, Dest: filepath.Join(srcDir, "{{.Name}}")},
	}

	// For rename test, use a simple non-template dest
	r.Action.Dest = destPath

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "rename" {
		t.Errorf("action = %q, want %q", result.Record.Action, "rename")
	}

	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading renamed file: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}
}

func TestUndoMove(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello")

	r := rule.Rule{
		Name:   "test-move",
		Action: rule.Action{Type: rule.ActionMove, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Now undo
	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	// Source should be restored
	data, err := os.ReadFile(fi.Path)
	if err != nil {
		t.Fatalf("source should be restored: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("restored content = %q, want %q", string(data), "hello")
	}

	// Dest should be gone
	if _, err := os.Stat(result.Record.Dest); !os.IsNotExist(err) {
		t.Error("dest should be removed after undo")
	}
}

func TestUndoCopy(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "hello")

	r := rule.Rule{
		Name:   "test-copy",
		Action: rule.Action{Type: rule.ActionCopy, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	// Copy should be removed
	if _, err := os.Stat(result.Record.Dest); !os.IsNotExist(err) {
		t.Error("copied file should be removed after undo")
	}

	// Source should still exist
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source should still exist after undo copy")
	}
}
