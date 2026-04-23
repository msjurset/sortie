package dispatcher

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/msjurset/sortie/internal/rule"
)

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

	result, err := disp.Dispatch(fi, r, nil, false)
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

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "open" {
		t.Errorf("action = %q, want %q", result.Record.Action, "open")
	}
}

func TestDispatchUnquarantine(t *testing.T) {
	if _, err := exec.LookPath("xattr"); err != nil {
		t.Skip("xattr not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "app.dmg", "fake disk image")

	exec.Command("xattr", "-w", "com.apple.quarantine", "0081;deadbeef;Safari;", fi.Path).Run()

	r := rule.Rule{
		Name:   "test-unquarantine",
		Action: rule.Action{Type: rule.ActionUnquarantine},
	}

	result, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "unquarantine" {
		t.Errorf("action = %q, want %q", result.Record.Action, "unquarantine")
	}

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

	_, err := disp.Dispatch(fi, r, nil, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}
}
