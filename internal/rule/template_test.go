package rule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpandTemplate(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 3, 18, 14, 30, 45, 0, time.UTC)

	fi := testFile(t, dir, "report.pdf", 1024, modTime)

	tests := []struct {
		name     string
		template string
		contains string
	}{
		{
			name:     "year and month in path",
			template: filepath.Join(dir, "out", "{{.Year}}", "{{.Month}}"),
			contains: filepath.Join(dir, "out", "2026", "03", "report.pdf"),
		},
		{
			name:     "date in filename",
			template: filepath.Join(dir, "out", "{{.Date}}_{{.Name}}{{.Ext}}"),
			contains: filepath.Join(dir, "out", "2026-03-18_report.pdf"),
		},
		{
			name:     "name and ext",
			template: filepath.Join(dir, "out", "{{.Name}}-backup{{.Ext}}"),
			contains: filepath.Join(dir, "out", "report-backup.pdf"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, fi)
			if err != nil {
				t.Fatalf("ExpandTemplate() error: %v", err)
			}
			if got != tt.contains {
				t.Errorf("ExpandTemplate() = %q, want %q", got, tt.contains)
			}
		})
	}
}

func TestExpandTemplateConflictResolution(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	modTime := time.Date(2026, 3, 18, 14, 30, 45, 0, time.UTC)

	// Create the source file
	fi := testFile(t, dir, "photo.jpg", 100, modTime)

	// Create an existing file at the destination
	existing := filepath.Join(destDir, "photo.jpg")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ExpandTemplate(filepath.Join(destDir, "{{.Name}}{{.Ext}}"), fi)
	if err != nil {
		t.Fatalf("ExpandTemplate() error: %v", err)
	}

	if !strings.Contains(got, "_001") {
		t.Errorf("expected conflict resolution suffix _001, got %q", got)
	}
}

func TestExpandString(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 3, 18, 14, 30, 45, 0, time.UTC)

	fi := testFile(t, dir, "report.pdf", 1024, modTime)

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "path variable",
			template: "cp '{{.Path}}' /backup/",
			want:     "cp '" + fi.Path + "' /backup/",
		},
		{
			name:     "name and ext",
			template: "{{.Name}}{{.Ext}}",
			want:     "report.pdf",
		},
		{
			name:     "date variables",
			template: "{{.Year}}-{{.Month}}-{{.Day}}",
			want:     "2026-03-18",
		},
		{
			name:     "empty template",
			template: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandString(tt.template, fi)
			if err != nil {
				t.Fatalf("ExpandString() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandTemplateWithCaptures(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 3, 18, 14, 30, 45, 0, time.UTC)

	fi := testFile(t, dir, "download.pdf", 1024, modTime)

	captures := map[string]string{
		"company": "acme",
		"date":    "2026-03-15",
	}

	t.Run("captures in dest path", func(t *testing.T) {
		tmpl := filepath.Join(dir, "out", "invoice-{{.Match.company}}-{{.Match.date}}{{.Ext}}")
		got, err := ExpandTemplate(tmpl, fi, captures)
		if err != nil {
			t.Fatalf("ExpandTemplate() error: %v", err)
		}
		want := filepath.Join(dir, "out", "invoice-acme-2026-03-15.pdf")
		if got != want {
			t.Errorf("ExpandTemplate() = %q, want %q", got, want)
		}
	})

	t.Run("nil captures works", func(t *testing.T) {
		tmpl := filepath.Join(dir, "out", "{{.Name}}{{.Ext}}")
		got, err := ExpandTemplate(tmpl, fi)
		if err != nil {
			t.Fatalf("ExpandTemplate() error: %v", err)
		}
		want := filepath.Join(dir, "out", "download.pdf")
		if got != want {
			t.Errorf("ExpandTemplate() = %q, want %q", got, want)
		}
	})
}

func TestExpandStringWithCaptures(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Date(2026, 3, 18, 14, 30, 45, 0, time.UTC)

	fi := testFile(t, dir, "report.pdf", 1024, modTime)

	captures := map[string]string{
		"company": "globex",
	}

	got, err := ExpandString("Invoice from {{.Match.company}}", fi, captures)
	if err != nil {
		t.Fatalf("ExpandString() error: %v", err)
	}
	if got != "Invoice from globex" {
		t.Errorf("ExpandString() = %q, want %q", got, "Invoice from globex")
	}
}
