package dispatcher

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msjurset/sortie/internal/history"
	"github.com/msjurset/sortie/internal/rule"
)

// --- Phase 1: File Operations ---

func TestDispatchExtractZip(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create a zip file with two entries
	zipPath := filepath.Join(srcDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"hello.txt":     "hello world",
		"sub/nested.txt": "nested content",
	})

	fi, err := rule.NewFileInfo(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(destDir, "extracted")
	r := rule.Rule{
		Name:   "test-extract",
		Action: rule.Action{Type: rule.ActionExtract, Dest: extractDir},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "extract" {
		t.Errorf("action = %q, want %q", result.Record.Action, "extract")
	}

	// Check extracted files
	data, err := os.ReadFile(filepath.Join(extractDir, "hello.txt"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}

	data, err = os.ReadFile(filepath.Join(extractDir, "sub/nested.txt"))
	if err != nil {
		t.Fatalf("reading nested extracted file: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("content = %q, want %q", string(data), "nested content")
	}
}

func TestDispatchExtractZipSkipsMacOSMetadata(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create a zip with __MACOSX metadata (like Finder creates)
	zipPath := filepath.Join(srcDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"hello.txt":              "hello world",
		"__MACOSX/._hello.txt":  "resource fork data",
		"__MACOSX/.DS_Store":    "ds store data",
		".DS_Store":             "root ds store",
	})

	fi, err := rule.NewFileInfo(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(destDir, "extracted")
	r := rule.Rule{
		Name:   "test-extract-no-macos",
		Action: rule.Action{Type: rule.ActionExtract, Dest: extractDir},
	}

	if _, err := disp.Dispatch(fi, r, false); err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Real file should exist
	if _, err := os.Stat(filepath.Join(extractDir, "hello.txt")); err != nil {
		t.Error("hello.txt should be extracted")
	}

	// __MACOSX directory should NOT exist
	if _, err := os.Stat(filepath.Join(extractDir, "__MACOSX")); !os.IsNotExist(err) {
		t.Error("__MACOSX should be skipped during extraction")
	}

	// .DS_Store should NOT exist
	if _, err := os.Stat(filepath.Join(extractDir, ".DS_Store")); !os.IsNotExist(err) {
		t.Error(".DS_Store should be skipped during extraction")
	}
}

func TestDispatchExtractTarGz(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create a .tar.gz file
	tgzPath := filepath.Join(srcDir, "test.tar.gz")
	createTestTarGz(t, tgzPath, map[string]string{
		"data.txt": "tar gz content",
	})

	fi, err := rule.NewFileInfo(tgzPath)
	if err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(destDir, "extracted")
	r := rule.Rule{
		Name:   "test-extract-tgz",
		Action: rule.Action{Type: rule.ActionExtract, Dest: extractDir},
	}

	if _, err := disp.Dispatch(fi, r, false); err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(extractDir, "data.txt"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(data) != "tar gz content" {
		t.Errorf("content = %q, want %q", string(data), "tar gz content")
	}
}

func TestUndoExtract(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	zipPath := filepath.Join(srcDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"a.txt": "data"})

	fi, err := rule.NewFileInfo(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(destDir, "extracted")
	r := rule.Rule{
		Name:   "test-extract",
		Action: rule.Action{Type: rule.ActionExtract, Dest: extractDir},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(extractDir); !os.IsNotExist(err) {
		t.Error("extracted dir should be removed after undo")
	}
}

func TestDispatchSymlink(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "original.txt", "original content")

	linkPath := filepath.Join(destDir, "link.txt")
	r := rule.Rule{
		Name:   "test-symlink",
		Action: rule.Action{Type: rule.ActionSymlink, Dest: linkPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "symlink" {
		t.Errorf("action = %q, want %q", result.Record.Action, "symlink")
	}

	// Source should still exist
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source file should still exist after symlink")
	}

	// Symlink should exist and point to source
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("reading symlink: %v", err)
	}

	absSource, _ := filepath.Abs(fi.Path)
	if target != absSource {
		t.Errorf("symlink target = %q, want %q", target, absSource)
	}

	// Reading through symlink should give original content
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("reading through symlink: %v", err)
	}
	if string(data) != "original content" {
		t.Errorf("content = %q, want %q", string(data), "original content")
	}
}

func TestUndoSymlink(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "original.txt", "content")

	linkPath := filepath.Join(destDir, "link.txt")
	r := rule.Rule{
		Name:   "test-symlink",
		Action: rule.Action{Type: rule.ActionSymlink, Dest: linkPath},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should be removed after undo")
	}

	// Source should still exist
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source should still exist after undo symlink")
	}
}

func TestDispatchChmod(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "script.sh", "#!/bin/sh\necho hi")

	r := rule.Rule{
		Name:   "test-chmod",
		Action: rule.Action{Type: rule.ActionChmod, Mode: "0755"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "chmod" {
		t.Errorf("action = %q, want %q", result.Record.Action, "chmod")
	}

	// Verify old mode was stored
	if result.Record.Dest != "0644" {
		t.Errorf("old mode = %q, want %q", result.Record.Dest, "0644")
	}

	// Verify new mode applied
	info, err := os.Stat(fi.Path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fmt.Sprintf("%04o", info.Mode().Perm()); perm != "0755" {
		t.Errorf("mode = %s, want 0755", perm)
	}
}

func TestUndoChmod(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "script.sh", "#!/bin/sh")

	r := rule.Rule{
		Name:   "test-chmod",
		Action: rule.Action{Type: rule.ActionChmod, Mode: "0755"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	info, err := os.Stat(fi.Path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fmt.Sprintf("%04o", info.Mode().Perm()); perm != "0644" {
		t.Errorf("mode after undo = %s, want 0644", perm)
	}
}

func TestDispatchChecksum(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "checksum me")

	r := rule.Rule{
		Name:   "test-checksum",
		Action: rule.Action{Type: rule.ActionChecksum, Algorithm: "sha256"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "checksum" {
		t.Errorf("action = %q, want %q", result.Record.Action, "checksum")
	}

	// Verify sidecar file exists
	sidecar := result.Record.Dest
	if !strings.HasSuffix(sidecar, ".sha256") {
		t.Errorf("sidecar = %q, should end with .sha256", sidecar)
	}

	data, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("reading sidecar: %v", err)
	}

	// Verify hash
	h := sha256.Sum256([]byte("checksum me"))
	expected := hex.EncodeToString(h[:]) + "  data.bin\n"
	if string(data) != expected {
		t.Errorf("sidecar content = %q, want %q", string(data), expected)
	}
}

func TestDispatchChecksumMD5(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "md5 me")

	r := rule.Rule{
		Name:   "test-checksum-md5",
		Action: rule.Action{Type: rule.ActionChecksum, Algorithm: "md5"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if !strings.HasSuffix(result.Record.Dest, ".md5") {
		t.Errorf("sidecar = %q, should end with .md5", result.Record.Dest)
	}
}

func TestDispatchChecksumDefault(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "default algo")

	r := rule.Rule{
		Name:   "test-checksum-default",
		Action: rule.Action{Type: rule.ActionChecksum},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	// Default algorithm is sha256
	if !strings.HasSuffix(result.Record.Dest, ".sha256") {
		t.Errorf("sidecar = %q, should end with .sha256", result.Record.Dest)
	}
}

func TestUndoChecksum(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "content")

	r := rule.Rule{
		Name:   "test-checksum",
		Action: rule.Action{Type: rule.ActionChecksum, Algorithm: "sha256"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if err := disp.Undo(result.Record); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(result.Record.Dest); !os.IsNotExist(err) {
		t.Error("sidecar should be removed after undo")
	}
}

// --- Phase 2: Shell Integration ---

func TestDispatchExec(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "input.txt", "hello")

	outPath := filepath.Join(destDir, "out.txt")
	r := rule.Rule{
		Name:   "test-exec",
		Action: rule.Action{Type: rule.ActionExec, Command: "cp '{{.Path}}' '" + outPath + "'"},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "exec" {
		t.Errorf("action = %q, want %q", result.Record.Action, "exec")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("exec output not found: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", string(data), "hello")
	}
}

func TestDispatchExecFailure(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "input.txt", "hello")

	r := rule.Rule{
		Name:   "test-exec-fail",
		Action: rule.Action{Type: rule.ActionExec, Command: "false"},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestUndoExecNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(makeRecord("exec", "/src", "/dest"))
	if err == nil {
		t.Fatal("expected error from undo exec")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

func TestUndoNotifyNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(makeRecord("notify", "/src", ""))
	if err == nil {
		t.Fatal("expected error from undo notify")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

func TestUndoUploadNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(makeRecord("upload", "/src", "s3://bucket/key"))
	if err == nil {
		t.Fatal("expected error from undo upload")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

func TestUndoTagNotReversible(t *testing.T) {
	disp, _ := newTestDispatcher(t)

	err := disp.Undo(makeRecord("tag", "/src", ""))
	if err == nil {
		t.Fatal("expected error from undo tag")
	}
	if !strings.Contains(err.Error(), "cannot undo") {
		t.Errorf("error = %q, want 'cannot undo'", err.Error())
	}
}

// --- Phase 3: Tool Wrappers ---

func TestDispatchResizeSips(t *testing.T) {
	if _, err := exec.LookPath("sips"); err != nil {
		t.Skip("sips not installed, skipping")
	}

	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create a minimal valid PNG (1x1 pixel)
	pngPath := filepath.Join(srcDir, "test.png")
	createMinimalPNG(t, pngPath)

	fi, err := rule.NewFileInfo(pngPath)
	if err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(destDir, "resized.png")
	r := rule.Rule{
		Name: "test-resize",
		Action: rule.Action{
			Type:  rule.ActionResize,
			Dest:  destPath,
			Width: 1,
		},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "resize" {
		t.Errorf("action = %q, want %q", result.Record.Action, "resize")
	}

	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("resized file should exist: %v", err)
	}

	// Source should still exist
	if _, err := os.Stat(fi.Path); err != nil {
		t.Error("source should still exist after resize")
	}
}

func TestUndoResize(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Create a dummy output file to undo
	destPath := filepath.Join(srcDir, "resized.png")
	if err := os.WriteFile(destPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := makeRecord("resize", "/original.png", destPath)
	if err := disp.Undo(rec); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("resized file should be removed after undo")
	}
}

// --- Extract Edge Cases ---

func TestDispatchExtractUnsupportedFormat(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.csv", "a,b,c")

	r := rule.Rule{
		Name:   "test-extract-bad",
		Action: rule.Action{Type: rule.ActionExtract, Dest: destDir},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for unsupported archive format")
	}
	if !strings.Contains(err.Error(), "unsupported archive format") {
		t.Errorf("error = %q, want 'unsupported archive format'", err.Error())
	}
}

func TestExtractZipSlipPrevention(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create a zip with a path traversal entry
	zipPath := filepath.Join(srcDir, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	// Write a file with a traversal path
	fw, err := w.Create("../../../tmp/evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("pwned"))
	w.Close()
	f.Close()

	extractDir := filepath.Join(destDir, "safe")
	err = doExtract(zipPath, extractDir)
	if err == nil {
		t.Fatal("expected error for zip slip path")
	}
	if !strings.Contains(err.Error(), "illegal path") {
		t.Errorf("error = %q, want 'illegal path'", err.Error())
	}
}

// --- Chmod Error Cases ---

func TestDispatchChmodInvalidMode(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "file.txt", "data")

	r := rule.Rule{
		Name:   "test-chmod-bad",
		Action: rule.Action{Type: rule.ActionChmod, Mode: "xyz"},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("error = %q, want 'invalid mode'", err.Error())
	}
}

// --- Checksum Error Cases ---

func TestDispatchChecksumBadAlgorithm(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "content")

	r := rule.Rule{
		Name:   "test-checksum-bad",
		Action: rule.Action{Type: rule.ActionChecksum, Algorithm: "sha512"},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("error = %q, want 'unsupported algorithm'", err.Error())
	}
}

func TestDispatchChecksumWithDest(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.bin", "hello")

	sidecar := filepath.Join(destDir, "custom.sha256")
	r := rule.Rule{
		Name:   "test-checksum-dest",
		Action: rule.Action{Type: rule.ActionChecksum, Algorithm: "sha256", Dest: sidecar},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Dest != sidecar {
		t.Errorf("dest = %q, want %q", result.Record.Dest, sidecar)
	}

	if _, err := os.Stat(sidecar); err != nil {
		t.Errorf("custom sidecar should exist: %v", err)
	}
}

// --- Notify Tests ---

func TestDispatchNotifyDesktop(t *testing.T) {
	if _, err := exec.LookPath("osascript"); err != nil {
		t.Skip("osascript not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "report.pdf", "content")

	r := rule.Rule{
		Name: "test-notify",
		Action: rule.Action{
			Type:    rule.ActionNotify,
			Title:   "Test",
			Message: "{{.Name}}{{.Ext}} arrived",
		},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "notify" {
		t.Errorf("action = %q, want %q", result.Record.Action, "notify")
	}
}

func TestDispatchNotifyDefaultTitle(t *testing.T) {
	if _, err := exec.LookPath("osascript"); err != nil {
		t.Skip("osascript not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "test.txt", "data")

	r := rule.Rule{
		Name: "test-notify-default",
		Action: rule.Action{
			Type:    rule.ActionNotify,
			Message: "hello",
		},
	}

	// Should use "sortie" as default title
	_, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}
}

// --- Convert Tests ---

func TestDispatchConvertMissingTool(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "video.mov", "fake video")

	r := rule.Rule{
		Name: "test-convert-notool",
		Action: rule.Action{
			Type: rule.ActionConvert,
			Tool: "nonexistent_tool_xyzzy",
			Args: "-i {{.Path}} {{.Dest}}",
			Dest: filepath.Join(destDir, "out.mp4"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool_xyzzy") {
		t.Errorf("error = %q, should mention missing tool", err.Error())
	}
}

func TestDispatchConvertRequiresTool(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "input.mov", "fake")

	r := rule.Rule{
		Name: "test-convert-no-tool-field",
		Action: rule.Action{
			Type: rule.ActionConvert,
			Args: "-i {{.Path}} {{.Dest}}",
			Dest: filepath.Join(destDir, "out.mp4"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error when tool field is empty")
	}
	if !strings.Contains(err.Error(), "requires tool") {
		t.Errorf("error = %q, want 'requires tool'", err.Error())
	}
}

// --- Watermark Tests ---

func TestDispatchWatermarkMissingOverlay(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "photo.jpg", "fake image")

	r := rule.Rule{
		Name: "test-watermark-no-overlay",
		Action: rule.Action{
			Type: rule.ActionWatermark,
			Dest: filepath.Join(destDir, "out.jpg"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing overlay")
	}
	if !strings.Contains(err.Error(), "requires overlay") {
		t.Errorf("error = %q, want 'requires overlay'", err.Error())
	}
}

func TestDispatchWatermarkMissingTool(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "photo.jpg", "fake image")

	r := rule.Rule{
		Name: "test-watermark-bad-tool",
		Action: rule.Action{
			Type:    rule.ActionWatermark,
			Tool:    "nonexistent_composite_xyzzy",
			Overlay: "/some/overlay.png",
			Dest:    filepath.Join(destDir, "out.jpg"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

// --- OCR Tests ---

func TestDispatchOCRMissingTool(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "scan.png", "fake image")

	r := rule.Rule{
		Name: "test-ocr-notool",
		Action: rule.Action{
			Type: rule.ActionOCR,
			Tool: "nonexistent_tesseract_xyzzy",
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

// --- Encrypt/Decrypt Tests ---

func TestDispatchEncryptMissingRecipient(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "secret.txt", "top secret")

	r := rule.Rule{
		Name: "test-encrypt-no-recipient",
		Action: rule.Action{
			Type: rule.ActionEncrypt,
			Dest: filepath.Join(destDir, "secret.txt.age"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing recipient")
	}
	if !strings.Contains(err.Error(), "requires recipient") {
		t.Errorf("error = %q, want 'requires recipient'", err.Error())
	}
}

func TestDispatchEncryptMissingTool(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "secret.txt", "top secret")

	r := rule.Rule{
		Name: "test-encrypt-notool",
		Action: rule.Action{
			Type:      rule.ActionEncrypt,
			Tool:      "nonexistent_age_xyzzy",
			Recipient: "age1test",
			Dest:      filepath.Join(destDir, "secret.txt.age"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestDispatchDecryptMissingTool(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "secret.txt.age", "encrypted")

	r := rule.Rule{
		Name: "test-decrypt-notool",
		Action: rule.Action{
			Type: rule.ActionDecrypt,
			Tool: "nonexistent_age_xyzzy",
			Key:  "/some/key.txt",
			Dest: filepath.Join(destDir, "secret.txt"),
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

// Test encrypt+decrypt round-trip with age if installed
func TestDispatchEncryptDecryptRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("age"); err != nil {
		t.Skip("age not installed, skipping")
	}
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen not installed, skipping")
	}

	srcDir := t.TempDir()
	destDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	// Generate a test key pair
	keyFile := filepath.Join(srcDir, "key.txt")
	out, err := exec.Command("age-keygen", "-o", keyFile).CombinedOutput()
	if err != nil {
		t.Fatalf("age-keygen: %s: %v", out, err)
	}

	// Extract public key from keygen output
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	var pubKey string
	for _, line := range strings.Split(string(keyData), "\n") {
		if strings.HasPrefix(line, "# public key: ") {
			pubKey = strings.TrimPrefix(line, "# public key: ")
			break
		}
	}
	if pubKey == "" {
		t.Fatal("could not extract public key from age-keygen output")
	}

	// Create a file to encrypt
	fi := testFileInfo(t, srcDir, "secret.txt", "super secret data")

	// Encrypt
	encPath := filepath.Join(destDir, "secret.txt.age")
	encRule := rule.Rule{
		Name: "test-encrypt",
		Action: rule.Action{
			Type:      rule.ActionEncrypt,
			Recipient: pubKey,
			Dest:      encPath,
		},
	}

	encResult, err := disp.Dispatch(fi, encRule, false)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	// Encrypted file should exist
	if _, err := os.Stat(encPath); err != nil {
		t.Fatalf("encrypted file missing: %v", err)
	}

	// Source should still exist
	if _, err := os.Stat(fi.Path); err != nil {
		t.Fatal("source should still exist after encrypt")
	}

	// Decrypt
	decPath := filepath.Join(destDir, "decrypted.txt")
	encFi, err := rule.NewFileInfo(encPath)
	if err != nil {
		t.Fatal(err)
	}

	decRule := rule.Rule{
		Name: "test-decrypt",
		Action: rule.Action{
			Type: rule.ActionDecrypt,
			Key:  keyFile,
			Dest: decPath,
		},
	}

	_, err = disp.Dispatch(encFi, decRule, false)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	// Verify decrypted content matches original
	data, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatalf("reading decrypted file: %v", err)
	}
	if string(data) != "super secret data" {
		t.Errorf("decrypted content = %q, want %q", string(data), "super secret data")
	}

	// Undo encrypt (should remove encrypted file)
	if err := disp.Undo(encResult.Record); err != nil {
		t.Fatalf("Undo encrypt error: %v", err)
	}
	if _, err := os.Stat(encPath); !os.IsNotExist(err) {
		t.Error("encrypted file should be removed after undo")
	}
}

// --- Upload Tests ---

func TestDispatchUploadMissingRemote(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "data.pdf", "content")

	r := rule.Rule{
		Name: "test-upload-no-remote",
		Action: rule.Action{
			Type: rule.ActionUpload,
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing remote")
	}
	if !strings.Contains(err.Error(), "requires remote") {
		t.Errorf("error = %q, want 'requires remote'", err.Error())
	}
}

func TestDetectUploadTool(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{"s3://bucket/key", "aws"},
		{"gs://bucket/key", "gsutil"},
		{"ftp://example.com/file", "aws"}, // default fallback
	}

	for _, tt := range tests {
		got := detectUploadTool(tt.remote)
		if got != tt.want {
			t.Errorf("detectUploadTool(%q) = %q, want %q", tt.remote, got, tt.want)
		}
	}
}

// --- Tag Tests ---

func TestDispatchTagMissingTags(t *testing.T) {
	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "doc.pdf", "content")

	r := rule.Rule{
		Name: "test-tag-empty",
		Action: rule.Action{
			Type: rule.ActionTag,
		},
	}

	_, err := disp.Dispatch(fi, r, false)
	if err == nil {
		t.Fatal("expected error for missing tags")
	}
	if !strings.Contains(err.Error(), "requires tags") {
		t.Errorf("error = %q, want 'requires tags'", err.Error())
	}
}

func TestBuildTagsPlist(t *testing.T) {
	plist := buildTagsPlist([]string{"Red", "Finance"})
	if !strings.Contains(plist, "<string>Red</string>") {
		t.Errorf("plist should contain Red tag: %s", plist)
	}
	if !strings.Contains(plist, "<string>Finance</string>") {
		t.Errorf("plist should contain Finance tag: %s", plist)
	}
	if !strings.Contains(plist, "<array>") || !strings.Contains(plist, "</array>") {
		t.Errorf("plist should wrap in array: %s", plist)
	}
}

func TestBuildTagsPlistXMLEscape(t *testing.T) {
	plist := buildTagsPlist([]string{`<script>alert("xss")</script>`})
	if strings.Contains(plist, "<script>") {
		t.Error("plist should escape XML special characters")
	}
	if !strings.Contains(plist, "&lt;script&gt;") {
		t.Errorf("plist should contain escaped tag: %s", plist)
	}
}

func TestDispatchTagWithXattr(t *testing.T) {
	if _, err := exec.LookPath("xattr"); err != nil {
		t.Skip("xattr not available, skipping")
	}

	srcDir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	fi := testFileInfo(t, srcDir, "invoice.pdf", "content")

	r := rule.Rule{
		Name: "test-tag",
		Action: rule.Action{
			Type: rule.ActionTag,
			Tags: []string{"Red", "Finance"},
		},
	}

	result, err := disp.Dispatch(fi, r, false)
	if err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	if result.Record.Action != "tag" {
		t.Errorf("action = %q, want %q", result.Record.Action, "tag")
	}

	// Verify xattr was set
	out, err := exec.Command("xattr", "-l", fi.Path).CombinedOutput()
	if err != nil {
		t.Fatalf("xattr -l: %v", err)
	}
	if !strings.Contains(string(out), "com.apple.metadata:_kMDItemUserTags") {
		t.Error("expected Finder tag xattr to be set")
	}
}

// --- Resize dimension helper ---

func TestResizeDimension(t *testing.T) {
	tests := []struct {
		name   string
		action rule.Action
		want   string
	}{
		{"percentage", rule.Action{Percentage: 50}, "50%"},
		{"width and height", rule.Action{Width: 800, Height: 600}, "800x600"},
		{"width only", rule.Action{Width: 1920}, "1920"},
		{"height only", rule.Action{Height: 1080}, "x1080"},
		{"none", rule.Action{}, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resizeDimension(tt.action)
			if got != tt.want {
				t.Errorf("resizeDimension() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Undo for output-producing actions (shared path) ---

func TestUndoConvert(t *testing.T) {
	dir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	outPath := filepath.Join(dir, "output.mp4")
	if err := os.WriteFile(outPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := disp.Undo(makeRecord("convert", "/input.mov", outPath)); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output should be removed after undo convert")
	}
}

func TestUndoWatermark(t *testing.T) {
	dir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	outPath := filepath.Join(dir, "watermarked.jpg")
	if err := os.WriteFile(outPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := disp.Undo(makeRecord("watermark", "/photo.jpg", outPath)); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output should be removed after undo watermark")
	}
}

func TestUndoOCR(t *testing.T) {
	dir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	outPath := filepath.Join(dir, "scan.txt")
	if err := os.WriteFile(outPath, []byte("extracted text"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := disp.Undo(makeRecord("ocr", "/scan.png", outPath)); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output should be removed after undo ocr")
	}
}

func TestUndoEncrypt(t *testing.T) {
	dir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	outPath := filepath.Join(dir, "secret.age")
	if err := os.WriteFile(outPath, []byte("encrypted"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := disp.Undo(makeRecord("encrypt", "/secret.txt", outPath)); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output should be removed after undo encrypt")
	}
}

func TestUndoDecrypt(t *testing.T) {
	dir := t.TempDir()
	disp, _ := newTestDispatcher(t)

	outPath := filepath.Join(dir, "decrypted.txt")
	if err := os.WriteFile(outPath, []byte("decrypted"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := disp.Undo(makeRecord("decrypt", "/secret.age", outPath)); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}

	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("output should be removed after undo decrypt")
	}
}

// --- Helpers ---

func makeRecord(action, src, dest string) history.Record {
	return history.Record{Action: action, Src: src, Dest: dest}
}

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func createTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
}

// createMinimalPNG writes a valid 1x1 red pixel PNG file.
func createMinimalPNG(t *testing.T, path string) {
	t.Helper()
	// Minimal valid PNG: 1x1 pixel, RGBA
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, // compressed data
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, // ...
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND chunk
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatal(err)
	}
}
