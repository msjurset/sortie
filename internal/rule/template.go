package rule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateData holds the variables available in destination path templates.
type TemplateData struct {
	Name  string // filename without extension
	Ext   string // extension including dot
	Path  string // full source file path
	Year  string // 4-digit year from mod time
	Month string // 2-digit month
	Day   string // 2-digit day
	Date  string // YYYY-MM-DD
	Time  string // HH-MM-SS
}

// ExpandTemplate expands a destination path template using the file's metadata.
// If the resulting path already exists, a counter suffix is appended.
func ExpandTemplate(tmpl string, fi FileInfo) (string, error) {
	name := fi.Info.Name()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	data := templateData(fi, base, ext)

	t, err := template.New("dest").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", tmpl, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", tmpl, err)
	}

	dest := buf.String()

	// If dest is a directory path (no extension in final segment or template
	// doesn't include filename), append the original filename.
	if !strings.Contains(filepath.Base(dest), ".") && ext != "" {
		dest = filepath.Join(dest, name)
	}

	return resolveConflict(dest, ext), nil
}

// ExpandString expands a template string using the file's metadata without
// conflict resolution or directory-append logic. Use this for non-path fields
// like Command, Message, Args, and Remote.
func ExpandString(tmpl string, fi FileInfo) (string, error) {
	if tmpl == "" {
		return "", nil
	}

	name := fi.Info.Name()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	data := templateData(fi, base, ext)

	t, err := template.New("str").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", tmpl, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %q: %w", tmpl, err)
	}

	return buf.String(), nil
}

func templateData(fi FileInfo, base, ext string) TemplateData {
	modTime := fi.Info.ModTime()
	return TemplateData{
		Name:  base,
		Ext:   ext,
		Path:  fi.Path,
		Year:  modTime.Format("2006"),
		Month: modTime.Format("01"),
		Day:   modTime.Format("02"),
		Date:  modTime.Format("2006-01-02"),
		Time:  modTime.Format("15-04-05"),
	}
}

// resolveConflict appends _001, _002, etc. if the destination already exists.
func resolveConflict(dest, ext string) string {
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}

	dir := filepath.Dir(dest)
	base := strings.TrimSuffix(filepath.Base(dest), ext)

	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%03d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}

	// Fallback: just return the original (overwrite scenario)
	return dest
}
