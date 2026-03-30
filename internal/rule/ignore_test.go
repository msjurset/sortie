package rule

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testIgnoreFile(t *testing.T, dir, name string) FileInfo {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := NewFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func TestShouldIgnoreGlobMatch(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "data.tmp")

	if !ShouldIgnore([]string{"*.tmp"}, nil, fi) {
		t.Error("*.tmp should match data.tmp")
	}
}

func TestShouldIgnoreGlobNoMatch(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "report.pdf")

	if ShouldIgnore([]string{"*.tmp"}, nil, fi) {
		t.Error("*.tmp should not match report.pdf")
	}
}

func TestShouldIgnoreExactMatch(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, ".DS_Store")

	if !ShouldIgnore([]string{".DS_Store"}, nil, fi) {
		t.Error(".DS_Store should match exactly")
	}
}

func TestShouldIgnoreCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "Thumbs.db")

	if !ShouldIgnore([]string{"thumbs.db"}, nil, fi) {
		t.Error("thumbs.db should match Thumbs.db case-insensitively")
	}
}

func TestShouldIgnoreNegation(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "important.log")

	// Global ignores all .log, local negates important.log
	global := []string{"*.log"}
	local := []string{"!important.log"}

	if ShouldIgnore(global, local, fi) {
		t.Error("!important.log should override *.log")
	}
}

func TestShouldIgnoreNegationDoesNotAffectOthers(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "debug.log")

	global := []string{"*.log"}
	local := []string{"!important.log"}

	if !ShouldIgnore(global, local, fi) {
		t.Error("debug.log should still be ignored (negation only for important.log)")
	}
}

func TestShouldIgnoreLocalAddsToGlobal(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "test.bak")

	global := []string{"*.tmp"}
	local := []string{"*.bak"}

	if !ShouldIgnore(global, local, fi) {
		t.Error("local *.bak should add to global patterns")
	}
}

func TestShouldIgnoreEmptyPatterns(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "file.txt")

	if ShouldIgnore(nil, nil, fi) {
		t.Error("no patterns should not ignore anything")
	}
}

func TestShouldIgnoreEmptyStringPatterns(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "file.txt")

	if ShouldIgnore([]string{""}, []string{""}, fi) {
		t.Error("empty string patterns should not match")
	}
}

func TestShouldIgnoreLocalNegatesGlobal(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "notes.tmp")

	// Global ignores *.tmp, local says "actually don't ignore *.tmp here"
	global := []string{"*.tmp"}
	local := []string{"!*.tmp"}

	if ShouldIgnore(global, local, fi) {
		t.Error("local !*.tmp should negate global *.tmp")
	}
}

func TestShouldIgnoreLastMatchWins(t *testing.T) {
	dir := t.TempDir()
	fi := testIgnoreFile(t, dir, "data.log")

	// Pattern order: ignore, un-ignore, re-ignore
	global := []string{"*.log", "!*.log", "*.log"}

	if !ShouldIgnore(global, nil, fi) {
		t.Error("last matching pattern (*.log) should win")
	}
}

func TestShouldIgnoreMultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	tests := []struct {
		name     string
		filename string
		global   []string
		local    []string
		want     bool
	}{
		{"DS_Store", ".DS_Store", []string{".DS_Store"}, nil, true},
		{"crdownload", "file.crdownload", []string{"*.crdownload"}, nil, true},
		{"office temp", "~$document.docx", []string{"~$*"}, nil, true},
		{"resource fork", "._photo.jpg", []string{"._*"}, nil, true},
		{"normal file", "report.pdf", []string{".DS_Store", "*.tmp"}, nil, false},
		{"local override", "keep.tmp", []string{"*.tmp"}, []string{"!keep.tmp"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := testFile(t, dir, tt.filename, 10, now)
			defer os.Remove(fi.Path)

			got := ShouldIgnore(tt.global, tt.local, fi)
			if got != tt.want {
				t.Errorf("ShouldIgnore(%v, %v, %q) = %v, want %v",
					tt.global, tt.local, tt.filename, got, tt.want)
			}
		})
	}
}
