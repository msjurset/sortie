package rule

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testFile(t *testing.T, dir, name string, size int, modTime time.Time) FileInfo {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	fi, err := NewFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func TestRuleMatches(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour) // 60 days ago

	tests := []struct {
		name    string
		rule    Rule
		file    string
		size    int
		modTime time.Time
		want    bool
	}{
		{
			name:    "extension match",
			rule:    Rule{Match: Match{Extensions: []string{".jpg", ".png"}}},
			file:    "photo.jpg",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "extension no match",
			rule:    Rule{Match: Match{Extensions: []string{".jpg", ".png"}}},
			file:    "doc.pdf",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "extension case insensitive",
			rule:    Rule{Match: Match{Extensions: []string{".JPG"}}},
			file:    "photo.jpg",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "glob match",
			rule:    Rule{Match: Match{Glob: "Screenshot*"}},
			file:    "Screenshot 2026-03-18.png",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "glob no match",
			rule:    Rule{Match: Match{Glob: "Screenshot*"}},
			file:    "photo.jpg",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "regex match",
			rule:    Rule{Match: Match{Regex: `(?i)docker|vscode`}},
			file:    "Docker-4.0.dmg",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "regex no match",
			rule:    Rule{Match: Match{Regex: `(?i)docker|vscode`}},
			file:    "firefox.dmg",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "min size match",
			rule:    Rule{Match: Match{MinSize: "1KB"}},
			file:    "big.bin",
			size:    2048,
			modTime: now,
			want:    true,
		},
		{
			name:    "min size no match",
			rule:    Rule{Match: Match{MinSize: "1KB"}},
			file:    "small.bin",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "max size match",
			rule:    Rule{Match: Match{MaxSize: "1KB"}},
			file:    "tiny.txt",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "max size no match",
			rule:    Rule{Match: Match{MaxSize: "1KB"}},
			file:    "huge.txt",
			size:    2048,
			modTime: now,
			want:    false,
		},
		{
			name:    "min age match",
			rule:    Rule{Match: Match{MinAge: "30d"}},
			file:    "old.txt",
			size:    100,
			modTime: old,
			want:    true,
		},
		{
			name:    "min age no match",
			rule:    Rule{Match: Match{MinAge: "30d"}},
			file:    "new.txt",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name: "combined conditions all match",
			rule: Rule{Match: Match{
				Extensions: []string{".pdf"},
				MinSize:    "1KB",
			}},
			file:    "report.pdf",
			size:    2048,
			modTime: now,
			want:    true,
		},
		{
			name: "combined conditions partial match",
			rule: Rule{Match: Match{
				Extensions: []string{".pdf"},
				MinSize:    "1KB",
			}},
			file:    "report.pdf",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "mime type match by extension",
			rule:    Rule{Match: Match{MimeType: "image/"}},
			file:    "photo.png",
			size:    100,
			modTime: now,
			want:    true,
		},
		{
			name:    "mime type no match",
			rule:    Rule{Match: Match{MimeType: "image/"}},
			file:    "doc.txt",
			size:    100,
			modTime: now,
			want:    false,
		},
		{
			name:    "mime type exact match",
			rule:    Rule{Match: Match{MimeType: "application/pdf"}},
			file:    "report.pdf",
			size:    100,
			modTime: now,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := testFile(t, dir, tt.file, tt.size, tt.modTime)
			got := tt.rule.Matches(fi)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
			// Clean up for next test
			os.Remove(fi.Path)
		})
	}
}

func TestFirstMatch(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	rules := []Rule{
		{Name: "images", Match: Match{Extensions: []string{".jpg", ".png"}}},
		{Name: "pdfs", Match: Match{Extensions: []string{".pdf"}}},
		{Name: "catch-all", Match: Match{Glob: "*"}},
	}

	tests := []struct {
		name     string
		file     string
		wantRule string
	}{
		{"matches first rule", "photo.jpg", "images"},
		{"matches second rule", "doc.pdf", "pdfs"},
		{"falls through to catch-all", "readme.txt", "catch-all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fi := testFile(t, dir, tt.file, 100, now)
			matched := FirstMatch(rules, fi)
			if matched == nil {
				t.Fatal("expected a match, got nil")
			}
			if matched.Name != tt.wantRule {
				t.Errorf("FirstMatch() matched %q, want %q", matched.Name, tt.wantRule)
			}
			os.Remove(fi.Path)
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"100B", 100, false},
		{"1KB", 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.5MB", int64(1.5 * 1024 * 1024), false},
		{"500", 500, false},
		{"bad", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseAge(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"30d", 30 * 24 * time.Hour, false},
		{"2h", 2 * time.Hour, false},
		{"45m", 45 * time.Minute, false},
		{"10s", 10 * time.Second, false},
		{"bad", 0, true},
		{"30", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAge(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAge(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseAge(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
