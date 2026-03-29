package rule

import (
	"strings"
	"testing"
)

func TestValidateEmptyRule(t *testing.T) {
	rules := []Rule{{Name: "empty-rule"}}
	findings := ValidateRules(rules, nil)

	if len(findings) == 0 {
		t.Fatal("expected findings for rule with no actions")
	}
	assertHasError(t, findings, "no actions defined")
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{"move no dest", Action{Type: ActionMove}, "requires dest"},
		{"copy no dest", Action{Type: ActionCopy}, "requires dest"},
		{"rename no dest", Action{Type: ActionRename}, "requires dest"},
		{"symlink no dest", Action{Type: ActionSymlink}, "requires dest"},
		{"extract no dest", Action{Type: ActionExtract}, "requires dest"},
		{"chmod no mode", Action{Type: ActionChmod}, "requires mode"},
		{"exec no command", Action{Type: ActionExec}, "requires command"},
		{"convert no tool", Action{Type: ActionConvert}, "requires tool"},
		{"resize no dims", Action{Type: ActionResize, Dest: "/out"}, "requires width, height, or percentage"},
		{"resize no dest", Action{Type: ActionResize, Width: 100}, "requires dest"},
		{"watermark no overlay", Action{Type: ActionWatermark, Dest: "/out"}, "requires overlay"},
		{"watermark no dest", Action{Type: ActionWatermark, Overlay: "/wm.png"}, "requires dest"},
		{"encrypt no recipient", Action{Type: ActionEncrypt}, "requires recipient"},
		{"upload no remote", Action{Type: ActionUpload}, "requires remote"},
		{"tag no tags", Action{Type: ActionTag}, "requires tags"},
		{"dedup no dest", Action{Type: ActionDeduplicate}, "requires dest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []Rule{{Name: "test", Action: tt.action}}
			findings := ValidateRules(rules, nil)
			assertHasError(t, findings, tt.wantErr)
		})
	}
}

func TestValidateInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		action  Action
		wantErr string
	}{
		{
			"bad chmod mode",
			Action{Type: ActionChmod, Mode: "abc"},
			"not a valid octal",
		},
		{
			"bad checksum algo",
			Action{Type: ActionChecksum, Algorithm: "sha512"},
			"unsupported checksum algorithm",
		},
		{
			"bad on_duplicate",
			Action{Type: ActionDeduplicate, Dest: "/dest", OnDuplicate: "ignore"},
			"must be \"skip\" or \"delete\"",
		},
		{
			"unknown action type",
			Action{Type: "frobnicate"},
			"unknown action type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []Rule{{Name: "test", Action: tt.action}}
			findings := ValidateRules(rules, nil)
			assertHasError(t, findings, tt.wantErr)
		})
	}
}

func TestValidateValidActions(t *testing.T) {
	// These should produce no errors (warnings are OK)
	tests := []struct {
		name   string
		action Action
	}{
		{"move", Action{Type: ActionMove, Dest: "/dest"}},
		{"copy", Action{Type: ActionCopy, Dest: "/dest"}},
		{"rename", Action{Type: ActionRename, Dest: "/dest"}},
		{"delete", Action{Type: ActionDelete}},
		{"compress", Action{Type: ActionCompress}},
		{"extract", Action{Type: ActionExtract, Dest: "/dest"}},
		{"symlink", Action{Type: ActionSymlink, Dest: "/dest"}},
		{"chmod", Action{Type: ActionChmod, Mode: "0755"}},
		{"checksum sha256", Action{Type: ActionChecksum, Algorithm: "sha256"}},
		{"checksum default", Action{Type: ActionChecksum}},
		{"exec", Action{Type: ActionExec, Command: "echo hi"}},
		{"notify", Action{Type: ActionNotify, Title: "hi", Message: "there"}},
		{"convert", Action{Type: ActionConvert, Tool: "ffmpeg", Args: "-i {{.Path}}", Dest: "/out"}},
		{"resize", Action{Type: ActionResize, Width: 800, Dest: "/out"}},
		{"watermark", Action{Type: ActionWatermark, Overlay: "/wm.png", Dest: "/out"}},
		{"ocr", Action{Type: ActionOCR}},
		{"encrypt", Action{Type: ActionEncrypt, Recipient: "age1..."}},
		{"decrypt", Action{Type: ActionDecrypt}},
		{"upload", Action{Type: ActionUpload, Remote: "s3://bucket/key"}},
		{"tag", Action{Type: ActionTag, Tags: []string{"Red"}}},
		{"open", Action{Type: ActionOpen}},
		{"dedup", Action{Type: ActionDeduplicate, Dest: "/dest"}},
		{"dedup skip", Action{Type: ActionDeduplicate, Dest: "/dest", OnDuplicate: "skip"}},
		{"dedup delete", Action{Type: ActionDeduplicate, Dest: "/dest", OnDuplicate: "delete"}},
		{"unquarantine", Action{Type: ActionUnquarantine}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []Rule{{Name: "test", Action: tt.action}}
			findings := ValidateRules(rules, nil)
			for _, f := range findings {
				if f.Severity == SeverityError {
					t.Errorf("unexpected error: %s", f.Message)
				}
			}
		})
	}
}

func TestValidateChainDeleteNotLast(t *testing.T) {
	rules := []Rule{{
		Name: "bad-chain",
		Actions: []Action{
			{Type: ActionDelete},
			{Type: ActionNotify, Title: "deleted", Message: "gone"},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasError(t, findings, "delete must be the last action")
}

func TestValidateChainCompressNotLast(t *testing.T) {
	rules := []Rule{{
		Name: "compress-chain",
		Actions: []Action{
			{Type: ActionCompress},
			{Type: ActionMove, Dest: "/archive"},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasWarning(t, findings, "compress removes the original")
}

func TestValidateChainDeduplicateNotLast(t *testing.T) {
	rules := []Rule{{
		Name: "dedup-chain",
		Actions: []Action{
			{Type: ActionDeduplicate, Dest: "/dest"},
			{Type: ActionTag, Tags: []string{"Red"}},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasWarning(t, findings, "conditional outcome")
}

func TestValidateChainOpenNotLast(t *testing.T) {
	rules := []Rule{{
		Name: "open-chain",
		Actions: []Action{
			{Type: ActionOpen},
			{Type: ActionMove, Dest: "/dest"},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasWarning(t, findings, "asynchronously")
}

func TestValidateChainConsecutiveMoves(t *testing.T) {
	rules := []Rule{{
		Name: "double-move",
		Actions: []Action{
			{Type: ActionMove, Dest: "/first"},
			{Type: ActionMove, Dest: "/second"},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasWarning(t, findings, "consecutive")
}

func TestValidateChainConsecutiveMoveRename(t *testing.T) {
	rules := []Rule{{
		Name: "move-rename",
		Actions: []Action{
			{Type: ActionMove, Dest: "/dest"},
			{Type: ActionRename, Dest: "/new-name"},
		},
	}}
	findings := ValidateRules(rules, nil)
	assertHasWarning(t, findings, "consecutive")
}

func TestValidateChainValidCombination(t *testing.T) {
	rules := []Rule{{
		Name: "good-chain",
		Actions: []Action{
			{Type: ActionNotify, Title: "New file", Message: "arrived"},
			{Type: ActionMove, Dest: "/dest"},
			{Type: ActionChmod, Mode: "0644"},
			{Type: ActionTag, Tags: []string{"Blue"}},
		},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))

	for _, f := range findings {
		if f.Severity == SeverityError {
			t.Errorf("unexpected error: %s", f.Message)
		}
		// Should have no warnings since /dest is not in /watched
		if f.Severity == SeverityWarning {
			t.Errorf("unexpected warning: %s", f.Message)
		}
	}
}

func TestValidateLoopRiskMoveToWatched(t *testing.T) {
	rules := []Rule{{
		Name:   "loop-risk",
		Action: Action{Type: ActionMove, Dest: "/watched/subdir/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirsRecursive("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateLoopRiskCopyToWatched(t *testing.T) {
	rules := []Rule{{
		Name:   "copy-loop",
		Action: Action{Type: ActionCopy, Dest: "/watched/backup/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirsRecursive("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateLoopRiskTemplateDest(t *testing.T) {
	rules := []Rule{{
		Name:   "template-loop",
		Action: Action{Type: ActionMove, Dest: "/watched/{{.Year}}/{{.Name}}{{.Ext}}"},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateNoLoopRiskDifferentDir(t *testing.T) {
	rules := []Rule{{
		Name:   "safe-move",
		Action: Action{Type: ActionMove, Dest: "/archive/files/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))

	for _, f := range findings {
		if strings.Contains(f.Message, "infinite dispatch loop") {
			t.Errorf("should not warn about loop risk: %s", f.Message)
		}
	}
}

func TestValidateLoopRiskExactMatch(t *testing.T) {
	rules := []Rule{{
		Name:   "same-dir",
		Action: Action{Type: ActionRename, Dest: "/watched/newname.txt"},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateNoLoopRiskSubdirNonRecursive(t *testing.T) {
	// Moving to a subdirectory of a non-recursive watch is safe
	rules := []Rule{{
		Name:   "safe-subdir",
		Action: Action{Type: ActionMove, Dest: "/watched/subdir/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))

	for _, f := range findings {
		if strings.Contains(f.Message, "infinite dispatch loop") {
			t.Errorf("should not warn about loop risk for subdirectory of non-recursive watch: %s", f.Message)
		}
	}
}

func TestValidateLoopRiskSubdirRecursive(t *testing.T) {
	// Moving to a subdirectory of a recursive watch IS risky
	rules := []Rule{{
		Name:   "recursive-loop",
		Action: Action{Type: ActionMove, Dest: "/watched/subdir/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirsRecursive("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateLoopRiskSameDirNonRecursive(t *testing.T) {
	// Moving to the exact same directory is always risky
	rules := []Rule{{
		Name:   "same-dir",
		Action: Action{Type: ActionMove, Dest: "/watched/file.txt"},
	}}
	findings := ValidateRules(rules, watchDirs("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

func TestValidateLoopRiskChain(t *testing.T) {
	rules := []Rule{{
		Name: "chain-loop",
		Actions: []Action{
			{Type: ActionNotify, Title: "hi", Message: "there"},
			{Type: ActionCopy, Dest: "/watched/backup/{{.Name}}{{.Ext}}"},
		},
	}}
	findings := ValidateRules(rules, watchDirsRecursive("/watched"))
	assertHasWarning(t, findings, "infinite dispatch loop")
}

// --- Helpers ---

func TestTemplateStaticPrefix(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/dest/file.txt", "/dest"},              // file dest — parent dir
		{"/dest/{{.Year}}/file.txt", "/dest"},     // template in middle
		{"{{.Name}}", ""},                         // template at start
		{"/a/b/c/{{.Name}}{{.Ext}}", "/a/b/c"},   // template at end
		{"/a/b/c", "/a/b/c"},                      // directory dest (no extension)
		{"/a/b/c/", "/a/b/c"},                     // trailing slash
		{"/watched/Images", "/watched/Images"},    // directory dest
		{"/watched/file.pdf", "/watched"},          // file dest
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := templateStaticPrefix(tt.path)
			if got != tt.want {
				t.Errorf("templateStaticPrefix(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsSubpath(t *testing.T) {
	tests := []struct {
		child, parent string
		want          bool
	}{
		{"/a/b", "/a", true},
		{"/a", "/a", true},
		{"/a/b/c", "/a", true},
		{"/b", "/a", false},
		{"/abc", "/a", false},    // not a subpath, just prefix match
		{"/a-b", "/a", false},    // not a subpath
	}

	for _, tt := range tests {
		t.Run(tt.child+"_in_"+tt.parent, func(t *testing.T) {
			got := isSubpath(tt.child, tt.parent)
			if got != tt.want {
				t.Errorf("isSubpath(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.want)
			}
		})
	}
}

func TestIsValidOctalMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"0755", true},
		{"644", true},
		{"0644", true},
		{"abc", false},
		{"0999", false}, // 9 is not octal
		{"07", false},   // too short
		{"07777", false}, // too long
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := isValidOctalMode(tt.mode)
			if got != tt.want {
				t.Errorf("isValidOctalMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func watchDirs(paths ...string) []WatchedDir {
	dirs := make([]WatchedDir, len(paths))
	for i, p := range paths {
		dirs[i] = WatchedDir{Path: p}
	}
	return dirs
}

func watchDirsRecursive(paths ...string) []WatchedDir {
	dirs := make([]WatchedDir, len(paths))
	for i, p := range paths {
		dirs[i] = WatchedDir{Path: p, Recursive: true}
	}
	return dirs
}

func assertHasError(t *testing.T, findings []Finding, substr string) {
	t.Helper()
	for _, f := range findings {
		if f.Severity == SeverityError && strings.Contains(f.Message, substr) {
			return
		}
	}
	t.Errorf("expected error containing %q, got findings: %v", substr, findings)
}

func assertHasWarning(t *testing.T, findings []Finding, substr string) {
	t.Helper()
	for _, f := range findings {
		if f.Severity == SeverityWarning && strings.Contains(f.Message, substr) {
			return
		}
	}
	t.Errorf("expected warning containing %q, got findings: %v", substr, findings)
}
