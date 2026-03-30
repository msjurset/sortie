package dispatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msjurset/sortie/internal/rule"
)

func TestDispatchChainNotifyThenMove(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "report.pdf", "content")

	r := rule.Rule{
		Name: "notify-and-move",
		Actions: []rule.Action{
			{Type: rule.ActionChmod, Mode: "0644"},
			{Type: rule.ActionMove, Dest: destDir},
		},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Last action result should be the move
	if result.Record.Action != "move" {
		t.Errorf("last action = %q, want %q", result.Record.Action, "move")
	}

	// Source should be gone
	if _, err := os.Stat(fi.Path); !os.IsNotExist(err) {
		t.Error("source should be removed after chain move")
	}

	// Dest should have the file
	destPath := filepath.Join(destDir, "report.pdf")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}

	// Both actions should be in history with the same ChainID
	records, err := disp.History.List(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) < 2 {
		t.Fatalf("expected at least 2 history records, got %d", len(records))
	}

	// Records are newest-first
	moveRec := records[0]
	chmodRec := records[1]

	if moveRec.ChainID == "" {
		t.Error("move record should have a ChainID")
	}
	if chmodRec.ChainID == "" {
		t.Error("chmod record should have a ChainID")
	}
	if moveRec.ChainID != chmodRec.ChainID {
		t.Errorf("chain IDs should match: %q != %q", moveRec.ChainID, chmodRec.ChainID)
	}
}

func TestDispatchChainMoveThenTag(t *testing.T) {
	if _, err := os.Stat("/usr/bin/xattr"); err != nil {
		t.Skip("xattr not available, skipping")
	}

	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "invoice.pdf", "invoice data")

	destPath := filepath.Join(destDir, "invoice.pdf")
	r := rule.Rule{
		Name: "move-and-tag",
		Actions: []rule.Action{
			{Type: rule.ActionMove, Dest: destPath},
			{Type: rule.ActionTag, Tags: []string{"Finance"}},
		},
	}

	_, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// File should be at dest
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("file should be at dest: %v", err)
	}

	// Tag should be applied to the file at its new location
	// (the chain updates fi.Path after move)
}

func TestDispatchChainFilePathUpdatesAfterMove(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "script.sh", "#!/bin/sh\necho hello")

	destPath := filepath.Join(destDir, "script.sh")
	r := rule.Rule{
		Name: "move-then-chmod",
		Actions: []rule.Action{
			{Type: rule.ActionMove, Dest: destPath},
			{Type: rule.ActionChmod, Mode: "0755"},
		},
	}

	_, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// File should be at dest with new permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o111 == 0 {
		t.Errorf("expected executable permissions, got %04o", perm)
	}
}

func TestDispatchChainDryRun(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "data")

	r := rule.Rule{
		Name: "chain-dry",
		Actions: []rule.Action{
			{Type: rule.ActionChmod, Mode: "0755"},
			{Type: rule.ActionMove, Dest: destDir},
		},
	}

	result, err := disp.Dispatch(fi, r, nil, true)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}

	// File should still be in original location
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source should still exist in dry-run mode")
	}
}

func TestDispatchChainStopsOnError(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "data")

	r := rule.Rule{
		Name: "chain-fail",
		Actions: []rule.Action{
			{Type: rule.ActionExec, Command: "false"}, // will fail
			{Type: rule.ActionChmod, Mode: "0755"},     // should not execute
		},
	}

	_, err := disp.Dispatch(fi, r, nil, false)
	if err == nil {
		t.Fatal("expected error from failing exec")
	}

	// Only 1 record should be in history (the failed exec)
	records, err := disp.History.List(0)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 history record, got %d", len(records))
	}
}

func TestDispatchSingleActionNoChainID(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "data")

	// Single action (via Actions field)
	r := rule.Rule{
		Name:   "single",
		Action: rule.Action{Type: rule.ActionMove, Dest: destDir},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Single actions should NOT have a ChainID
	if result.Record.ChainID != "" {
		t.Errorf("single action should not have ChainID, got %q", result.Record.ChainID)
	}
}

func TestDispatchEmptyActionsError(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "data")

	r := rule.Rule{Name: "empty"}

	_, err := disp.Dispatch(fi, r, nil, false)
	if err == nil {
		t.Fatal("expected error for rule with no actions")
	}
	if !strings.Contains(err.Error(), "no actions") {
		t.Errorf("error = %q, want 'no actions'", err.Error())
	}
}

func TestResolvedActions(t *testing.T) {
	// Singular Action field
	r1 := rule.Rule{Action: rule.Action{Type: rule.ActionMove, Dest: "/dest"}}
	if actions := r1.ResolvedActions(); len(actions) != 1 || actions[0].Type != rule.ActionMove {
		t.Errorf("singular Action should resolve to 1-element list")
	}

	// Plural Actions field
	r2 := rule.Rule{Actions: []rule.Action{
		{Type: rule.ActionNotify, Title: "hi"},
		{Type: rule.ActionMove, Dest: "/dest"},
	}}
	if actions := r2.ResolvedActions(); len(actions) != 2 {
		t.Errorf("plural Actions should resolve to 2-element list, got %d", len(actions))
	}

	// Both set — Actions takes precedence
	r3 := rule.Rule{
		Action:  rule.Action{Type: rule.ActionDelete},
		Actions: []rule.Action{{Type: rule.ActionMove, Dest: "/dest"}},
	}
	if actions := r3.ResolvedActions(); len(actions) != 1 || actions[0].Type != rule.ActionMove {
		t.Errorf("Actions should take precedence over Action")
	}

	// Neither set
	r4 := rule.Rule{Name: "empty"}
	if actions := r4.ResolvedActions(); actions != nil {
		t.Errorf("empty rule should resolve to nil, got %v", actions)
	}
}

func TestUndoChainRecords(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "doc.txt", "important")

	destPath := filepath.Join(destDir, "doc.txt")
	r := rule.Rule{
		Name: "chmod-then-copy",
		Actions: []rule.Action{
			{Type: rule.ActionChmod, Mode: "0755"},
			{Type: rule.ActionCopy, Dest: destPath},
		},
	}

	_, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Copy should exist
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("copy should exist: %v", err)
	}

	// Undo the copy (last action in chain)
	records, err := disp.History.List(0)
	if err != nil {
		t.Fatal(err)
	}

	// Undo newest record (copy)
	copyRec := records[0]
	if err := disp.Undo(copyRec); err != nil {
		t.Fatalf("Undo copy: %v", err)
	}

	// Copy should be removed
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("copy should be removed after undo")
	}

	// Undo the chmod
	chmodRec := records[1]
	if err := disp.Undo(chmodRec); err != nil {
		t.Fatalf("Undo chmod: %v", err)
	}
}
