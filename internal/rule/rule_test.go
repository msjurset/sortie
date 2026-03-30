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

func TestContentMatch(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	// Create a file with known content
	contentFile := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(contentFile, []byte("This is an Invoice #12345 for payment"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(contentFile, now, now); err != nil {
		t.Fatal(err)
	}
	fi, err := NewFileInfo(contentFile)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("case-insensitive substring match", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{Content: "invoice"}}
		if !r.Matches(fi) {
			t.Error("'invoice' should match 'Invoice' case-insensitively")
		}
	})

	t.Run("substring no match", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{Content: "receipt"}}
		if r.Matches(fi) {
			t.Error("'receipt' should not match")
		}
	})

	t.Run("regex match", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{ContentRegex: `#\d{5}`}}
		if !r.Matches(fi) {
			t.Error("regex #\\d{5} should match '#12345'")
		}
	})

	t.Run("regex no match", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{ContentRegex: `#\d{6}`}}
		if r.Matches(fi) {
			t.Error("regex #\\d{6} should not match '#12345'")
		}
	})

	t.Run("content AND extension", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{Extensions: []string{".pdf"}, Content: "invoice"}}
		if !r.Matches(fi) {
			t.Error("should match both extension and content")
		}

		r2 := Rule{Name: "test", Match: Match{Extensions: []string{".txt"}, Content: "invoice"}}
		if r2.Matches(fi) {
			t.Error("wrong extension should cause no match despite content match")
		}
	})

	t.Run("content_bytes limit", func(t *testing.T) {
		// Content is at beginning, limit should still find it
		r := Rule{Name: "test", Match: Match{Content: "invoice", ContentBytes: 10}}
		// "This is an" = 10 bytes, doesn't contain "invoice"
		if r.Matches(fi) {
			t.Error("content_bytes=10 should not reach 'invoice' at position 15+")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		emptyPath := filepath.Join(dir, "empty.txt")
		os.WriteFile(emptyPath, nil, 0o644)
		os.Chtimes(emptyPath, now, now)
		emptyFi, _ := NewFileInfo(emptyPath)

		r := Rule{Name: "test", Match: Match{Content: "anything"}}
		if r.Matches(emptyFi) {
			t.Error("empty file should not match content search")
		}
	})
}

func TestContentCaptures(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	content := "Invoice #12345 from Acme Corp dated 2026-03-15"
	path := filepath.Join(dir, "invoice.pdf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	fi, err := NewFileInfo(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("named groups captured", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{
			ContentRegex: `(?P<company>Acme Corp).*(?P<date>\d{4}-\d{2}-\d{2})`,
		}}
		ok, captures := r.MatchWithCaptures(fi)
		if !ok {
			t.Fatal("expected match")
		}
		if captures["company"] != "Acme Corp" {
			t.Errorf("company = %q, want %q", captures["company"], "Acme Corp")
		}
		if captures["date"] != "2026-03-15" {
			t.Errorf("date = %q, want %q", captures["date"], "2026-03-15")
		}
	})

	t.Run("no named groups returns nil captures", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{
			ContentRegex: `Invoice #\d+`,
		}}
		ok, captures := r.MatchWithCaptures(fi)
		if !ok {
			t.Fatal("expected match")
		}
		if captures != nil {
			t.Errorf("expected nil captures for unnamed groups, got %v", captures)
		}
	})

	t.Run("captures with content AND content_regex", func(t *testing.T) {
		r := Rule{Name: "test", Match: Match{
			Content:      "invoice",
			ContentRegex: `from (?P<company>[A-Za-z ]+) dated`,
		}}
		ok, captures := r.MatchWithCaptures(fi)
		if !ok {
			t.Fatal("expected match")
		}
		if captures["company"] != "Acme Corp" {
			t.Errorf("company = %q, want %q", captures["company"], "Acme Corp")
		}
	})

	t.Run("FindMatches propagates captures", func(t *testing.T) {
		rules := []Rule{
			{Name: "invoices", Match: Match{
				ContentRegex: `from (?P<company>[A-Za-z ]+) dated (?P<date>\d{4}-\d{2}-\d{2})`,
			}},
		}
		results := FindMatches(rules, fi)
		if len(results) != 1 {
			t.Fatalf("expected 1 match, got %d", len(results))
		}
		if results[0].Captures["company"] != "Acme Corp" {
			t.Errorf("company = %q, want %q", results[0].Captures["company"], "Acme Corp")
		}
		if results[0].Captures["date"] != "2026-03-15" {
			t.Errorf("date = %q, want %q", results[0].Captures["date"], "2026-03-15")
		}
	})

	t.Run("optional group captured independently", func(t *testing.T) {
		// Company appears after the date anchor — a single FindStringSubmatch
		// with an optional group would miss it due to RE2 leftmost match.
		// The independent retry should find it.
		content2 := "Invoice dated 2026-04-01 from Oracle for cloud services"
		path2 := filepath.Join(dir, "oracle-invoice.pdf")
		if err := os.WriteFile(path2, []byte(content2), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path2, now, now); err != nil {
			t.Fatal(err)
		}
		fi2, err := NewFileInfo(path2)
		if err != nil {
			t.Fatal(err)
		}

		r := Rule{Name: "test", Match: Match{
			ContentRegex: `(?P<company>Oracle|AWS)?.*(?P<date>\d{4}-\d{2}-\d{2})`,
		}}
		ok, captures := r.MatchWithCaptures(fi2)
		if !ok {
			t.Fatal("expected match")
		}
		if captures["date"] != "2026-04-01" {
			t.Errorf("date = %q, want %q", captures["date"], "2026-04-01")
		}
		if captures["company"] != "Oracle" {
			t.Errorf("company = %q, want %q (should be found via independent search)", captures["company"], "Oracle")
		}
	})

	t.Run("missing optional group stays empty", func(t *testing.T) {
		// Unknown company should result in empty capture
		content3 := "Invoice dated 2026-05-01 from Unknown Corp"
		path3 := filepath.Join(dir, "unknown-invoice.pdf")
		if err := os.WriteFile(path3, []byte(content3), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path3, now, now); err != nil {
			t.Fatal(err)
		}
		fi3, err := NewFileInfo(path3)
		if err != nil {
			t.Fatal(err)
		}

		r := Rule{Name: "test", Match: Match{
			ContentRegex: `(?P<company>Oracle|AWS)?.*(?P<date>\d{4}-\d{2}-\d{2})`,
		}}
		ok, captures := r.MatchWithCaptures(fi3)
		if !ok {
			t.Fatal("expected match")
		}
		if captures["date"] != "2026-05-01" {
			t.Errorf("date = %q, want %q", captures["date"], "2026-05-01")
		}
		if captures["company"] != "" {
			t.Errorf("company = %q, want empty (unknown vendor)", captures["company"])
		}
	})
}

func TestIsPDF(t *testing.T) {
	dir := t.TempDir()

	t.Run("real PDF magic bytes", func(t *testing.T) {
		path := filepath.Join(dir, "real.pdf")
		os.WriteFile(path, []byte("%PDF-1.4 fake pdf content"), 0o644)
		if !isPDF(path) {
			t.Error("expected isPDF=true for file with %PDF- magic bytes")
		}
	})

	t.Run("plain text with pdf extension", func(t *testing.T) {
		path := filepath.Join(dir, "fake.pdf")
		os.WriteFile(path, []byte("This is plain text, not a PDF"), 0o644)
		if isPDF(path) {
			t.Error("expected isPDF=false for plain text file with .pdf extension")
		}
	})

	t.Run("non-pdf file", func(t *testing.T) {
		path := filepath.Join(dir, "readme.txt")
		os.WriteFile(path, []byte("hello world"), 0o644)
		if isPDF(path) {
			t.Error("expected isPDF=false for .txt file")
		}
	})
}

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-03-15", "2026-03-15"},
		{"28-FEB-2026", "2026-02-28"},
		{"28-Feb-2026", "2026-02-28"},
		{"March 6, 2026", "2026-03-06"},
		{"March 15, 2026", "2026-03-15"},
		{"Jan 1, 2026", "2026-01-01"},
		{"03/15/2026", "2026-03-15"},
		{"3/6/2026", "2026-03-06"},
		{"not a date", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDate(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractNamedGroup(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		group   string
		wantNil bool
	}{
		{"simple group", `(?P<date>\d{4}-\d{2}-\d{2})`, "date", false},
		{"group with character class parens", `(?P<val>[()]+)`, "val", false},
		{"group with escaped paren", `(?P<val>\(\))`, "val", false},
		{"nested groups", `(?P<outer>abc(?:def))`, "outer", false},
		{"missing group", `(?P<date>\d+)`, "company", true},
		{"group with brackets and parens", `(?P<x>[a-z(]+)`, "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := extractNamedGroup(tt.pattern, tt.group)
			if tt.wantNil && re != nil {
				t.Errorf("expected nil, got %v", re)
			}
			if !tt.wantNil && re == nil {
				t.Errorf("expected non-nil regex for group %q in pattern %q", tt.group, tt.pattern)
			}
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
			if matched.Rule.Name != tt.wantRule {
				t.Errorf("FirstMatch() matched %q, want %q", matched.Rule.Name, tt.wantRule)
			}
			os.Remove(fi.Path)
		})
	}
}

func TestFirstMatchPriority(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	t.Run("higher priority wins over declaration order", func(t *testing.T) {
		rules := []Rule{
			{Name: "catch-all", Priority: 0, Match: Match{Glob: "*"}, Action: Action{Type: "move"}},
			{Name: "pdfs", Priority: 10, Match: Match{Extensions: []string{".pdf"}}, Action: Action{Type: "move"}},
		}
		fi := testFile(t, dir, "doc.pdf", 100, now)
		defer os.Remove(fi.Path)

		matched := FirstMatch(rules, fi)
		if matched == nil || matched.Rule.Name != "pdfs" {
			t.Errorf("expected pdfs (priority 10), got %v", matched)
		}
	})

	t.Run("equal priority preserves declaration order", func(t *testing.T) {
		rules := []Rule{
			{Name: "first", Priority: 5, Match: Match{Glob: "*"}, Action: Action{Type: "move"}},
			{Name: "second", Priority: 5, Match: Match{Glob: "*"}, Action: Action{Type: "copy"}},
		}
		fi := testFile(t, dir, "test.txt", 100, now)
		defer os.Remove(fi.Path)

		matched := FirstMatch(rules, fi)
		if matched == nil || matched.Rule.Name != "first" {
			t.Errorf("expected first (same priority, declared first), got %v", matched)
		}
	})

	t.Run("zero priority preserves original order", func(t *testing.T) {
		rules := []Rule{
			{Name: "images", Match: Match{Extensions: []string{".jpg"}}},
			{Name: "catch-all", Match: Match{Glob: "*"}},
		}
		fi := testFile(t, dir, "photo.jpg", 100, now)
		defer os.Remove(fi.Path)

		matched := FirstMatch(rules, fi)
		if matched == nil || matched.Rule.Name != "images" {
			t.Errorf("expected images (declared first, both priority 0), got %v", matched)
		}
	})

	t.Run("per-dir rules win at same priority via slice position", func(t *testing.T) {
		// Per-dir rules come first in the merged slice
		rules := []Rule{
			{Name: "per-dir-rule", Priority: 0, Match: Match{Glob: "*"}, Action: Action{Type: "move"}},
			{Name: "global-rule", Priority: 0, Match: Match{Glob: "*"}, Action: Action{Type: "copy"}},
		}
		fi := testFile(t, dir, "file.txt", 100, now)
		defer os.Remove(fi.Path)

		matched := FirstMatch(rules, fi)
		if matched == nil || matched.Rule.Name != "per-dir-rule" {
			t.Errorf("expected per-dir-rule (first in slice at same priority), got %v", matched)
		}
	})

	t.Run("negative priority sorts after zero", func(t *testing.T) {
		rules := []Rule{
			{Name: "fallback", Priority: -1, Match: Match{Glob: "*"}},
			{Name: "normal", Priority: 0, Match: Match{Glob: "*"}},
		}
		fi := testFile(t, dir, "test.txt", 100, now)
		defer os.Remove(fi.Path)

		matched := FirstMatch(rules, fi)
		if matched == nil || matched.Rule.Name != "normal" {
			t.Errorf("expected normal (priority 0 > -1), got %v", matched)
		}
	})
}

func TestFindMatchesContinue(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	t.Run("default stops at first match", func(t *testing.T) {
		rules := []Rule{
			{Name: "first", Match: Match{Glob: "*"}, Action: Action{Type: "notify"}},
			{Name: "second", Match: Match{Glob: "*"}, Action: Action{Type: "move", Dest: "/dest"}},
		}
		fi := testFile(t, dir, "test.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].Rule.Name != "first" {
			t.Errorf("expected first, got %s", matches[0].Rule.Name)
		}
	})

	t.Run("continue allows fall-through", func(t *testing.T) {
		rules := []Rule{
			{Name: "notify", Continue: true, Match: Match{Glob: "*"}, Action: Action{Type: "notify"}},
			{Name: "move", Match: Match{Glob: "*"}, Action: Action{Type: "move", Dest: "/dest"}},
		}
		fi := testFile(t, dir, "test2.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matches))
		}
		if matches[0].Rule.Name != "notify" {
			t.Errorf("first match should be notify, got %s", matches[0].Rule.Name)
		}
		if matches[1].Rule.Name != "move" {
			t.Errorf("second match should be move, got %s", matches[1].Rule.Name)
		}
	})

	t.Run("continue stops at non-continue rule", func(t *testing.T) {
		rules := []Rule{
			{Name: "first", Continue: true, Match: Match{Glob: "*"}, Action: Action{Type: "notify"}},
			{Name: "second", Match: Match{Glob: "*"}, Action: Action{Type: "tag"}},
			{Name: "third", Match: Match{Glob: "*"}, Action: Action{Type: "move", Dest: "/dest"}},
		}
		fi := testFile(t, dir, "test3.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches (stop at second which has no continue), got %d", len(matches))
		}
		if matches[1].Rule.Name != "second" {
			t.Errorf("second match should be second, got %s", matches[1].Rule.Name)
		}
	})

	t.Run("continue skips non-matching rules", func(t *testing.T) {
		rules := []Rule{
			{Name: "notify-all", Continue: true, Match: Match{Glob: "*"}, Action: Action{Type: "notify"}},
			{Name: "pdfs-only", Match: Match{Extensions: []string{".pdf"}}, Action: Action{Type: "move", Dest: "/dest"}},
			{Name: "catch-all", Match: Match{Glob: "*"}, Action: Action{Type: "delete"}},
		}
		fi := testFile(t, dir, "test4.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches (notify + catch-all, skip pdfs), got %d", len(matches))
		}
		if matches[0].Rule.Name != "notify-all" {
			t.Errorf("first should be notify-all, got %s", matches[0].Rule.Name)
		}
		if matches[1].Rule.Name != "catch-all" {
			t.Errorf("second should be catch-all, got %s", matches[1].Rule.Name)
		}
	})

	t.Run("continue with priority", func(t *testing.T) {
		rules := []Rule{
			{Name: "low-pri", Priority: 0, Match: Match{Glob: "*"}, Action: Action{Type: "delete"}},
			{Name: "high-pri", Priority: 10, Continue: true, Match: Match{Glob: "*"}, Action: Action{Type: "notify"}},
		}
		fi := testFile(t, dir, "test5.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matches))
		}
		// high-pri fires first (sorted by priority), then falls through to low-pri
		if matches[0].Rule.Name != "high-pri" {
			t.Errorf("first should be high-pri, got %s", matches[0].Rule.Name)
		}
		if matches[1].Rule.Name != "low-pri" {
			t.Errorf("second should be low-pri, got %s", matches[1].Rule.Name)
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		rules := []Rule{
			{Name: "pdfs", Match: Match{Extensions: []string{".pdf"}}},
		}
		fi := testFile(t, dir, "test6.txt", 100, now)
		defer os.Remove(fi.Path)

		matches := FindMatches(rules, fi)
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
	})
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
