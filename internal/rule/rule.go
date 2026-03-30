package rule

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Rule defines a file matching rule and its associated action(s).
type Rule struct {
	Name     string   `yaml:"name"`
	Match    Match    `yaml:"match"`
	Action   Action   `yaml:"action,omitempty"`   // single action (backwards-compatible)
	Actions  []Action `yaml:"actions,omitempty"`   // action chain (evaluated in order)
	Priority int      `yaml:"priority,omitempty"`
	Cooldown string   `yaml:"cooldown,omitempty"` // minimum interval between rule triggers e.g. "5s", "1m"
	Continue bool     `yaml:"continue,omitempty"` // if true, keep evaluating subsequent rules after this one matches
}

// ResolvedActions returns the list of actions to execute. If Actions is set,
// it is returned directly. Otherwise, the single Action field is wrapped in a
// one-element slice. This allows both singular and plural forms in YAML.
func (r *Rule) ResolvedActions() []Action {
	if len(r.Actions) > 0 {
		return r.Actions
	}
	if r.Action.Type != "" {
		return []Action{r.Action}
	}
	return nil
}

// Match defines the conditions for a rule. All specified conditions must be
// true (AND logic).
type Match struct {
	Extensions   []string `yaml:"extensions,omitempty"`
	Glob         string   `yaml:"glob,omitempty"`
	Regex        string   `yaml:"regex,omitempty"`
	MinSize      string   `yaml:"min_size,omitempty"`
	MaxSize      string   `yaml:"max_size,omitempty"`
	MinAge       string   `yaml:"min_age,omitempty"`
	MaxAge       string   `yaml:"max_age,omitempty"`
	MimeType     string   `yaml:"mime_type,omitempty"`
	Content      string   `yaml:"content,omitempty"`       // case-insensitive substring search on file content
	ContentRegex string   `yaml:"content_regex,omitempty"` // regex search on file content
	ContentBytes int      `yaml:"content_bytes,omitempty"` // max bytes to read (default 65536)
}

// ActionType represents the type of action to perform on a matched file.
type ActionType string

const (
	ActionMove      ActionType = "move"
	ActionCopy      ActionType = "copy"
	ActionRename    ActionType = "rename"
	ActionDelete    ActionType = "delete"
	ActionCompress  ActionType = "compress"
	ActionExtract   ActionType = "extract"
	ActionSymlink   ActionType = "symlink"
	ActionChmod     ActionType = "chmod"
	ActionChecksum  ActionType = "checksum"
	ActionExec      ActionType = "exec"
	ActionNotify    ActionType = "notify"
	ActionConvert   ActionType = "convert"
	ActionResize    ActionType = "resize"
	ActionWatermark ActionType = "watermark"
	ActionOCR       ActionType = "ocr"
	ActionEncrypt   ActionType = "encrypt"
	ActionDecrypt   ActionType = "decrypt"
	ActionUpload        ActionType = "upload"
	ActionTag           ActionType = "tag"
	ActionOpen          ActionType = "open"
	ActionDeduplicate   ActionType = "deduplicate"
	ActionUnquarantine  ActionType = "unquarantine"
)

// Action defines what to do with a matched file.
type Action struct {
	Type       ActionType `yaml:"type"`
	Dest       string     `yaml:"dest,omitempty"`
	Mode       string     `yaml:"mode,omitempty"`       // chmod: permission string e.g. "0644"
	Algorithm  string     `yaml:"algorithm,omitempty"`   // checksum: "sha256", "md5", "sha1"
	Command    string     `yaml:"command,omitempty"`     // exec: shell command template
	Title      string     `yaml:"title,omitempty"`       // notify: notification title
	Message    string     `yaml:"message,omitempty"`     // notify: body text or webhook URL
	Tool       string     `yaml:"tool,omitempty"`        // convert/resize/watermark/ocr/encrypt/decrypt: binary name
	Args       string     `yaml:"args,omitempty"`        // convert/resize/watermark: extra arguments template
	Width      int        `yaml:"width,omitempty"`       // resize: target width in pixels
	Height     int        `yaml:"height,omitempty"`      // resize: target height in pixels
	Percentage int        `yaml:"percentage,omitempty"`  // resize: scale percentage
	Overlay    string     `yaml:"overlay,omitempty"`     // watermark: path to overlay image
	Gravity    string     `yaml:"gravity,omitempty"`     // watermark: placement e.g. "center", "southeast"
	Language   string     `yaml:"language,omitempty"`    // ocr: tesseract language code
	Recipient  string     `yaml:"recipient,omitempty"`   // encrypt: age/gpg recipient
	Key        string     `yaml:"key,omitempty"`         // decrypt: key file path
	Tags        []string `yaml:"tags,omitempty"`         // tag: macOS Finder tags
	Remote      string   `yaml:"remote,omitempty"`       // upload: destination URI e.g. "s3://bucket/path"
	App         string   `yaml:"app,omitempty"`          // open: application name e.g. "VLC", "Preview"
	OnDuplicate string   `yaml:"on_duplicate,omitempty"` // deduplicate: "skip" (default) or "delete"
}

// FileInfo wraps os.FileInfo with the full file path.
type FileInfo struct {
	Path string
	Info os.FileInfo
}

// NewFileInfo creates a FileInfo from a path by stat-ing the file.
func NewFileInfo(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{Path: path, Info: info}, nil
}

// Matches returns true if the file satisfies all conditions in the rule.
func (r *Rule) Matches(fi FileInfo) bool {
	if len(r.Match.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(fi.Info.Name()))
		found := false
		for _, e := range r.Match.Extensions {
			if strings.ToLower(e) == ext {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if r.Match.Glob != "" {
		matched, err := filepath.Match(r.Match.Glob, fi.Info.Name())
		if err != nil || !matched {
			return false
		}
	}

	if r.Match.Regex != "" {
		re, err := regexp.Compile(r.Match.Regex)
		if err != nil || !re.MatchString(fi.Info.Name()) {
			return false
		}
	}

	if r.Match.MinSize != "" {
		sz, err := ParseSize(r.Match.MinSize)
		if err != nil || fi.Info.Size() < sz {
			return false
		}
	}

	if r.Match.MaxSize != "" {
		sz, err := ParseSize(r.Match.MaxSize)
		if err != nil || fi.Info.Size() > sz {
			return false
		}
	}

	now := time.Now()

	if r.Match.MinAge != "" {
		d, err := ParseAge(r.Match.MinAge)
		if err != nil || now.Sub(fi.Info.ModTime()) < d {
			return false
		}
	}

	if r.Match.MaxAge != "" {
		d, err := ParseAge(r.Match.MaxAge)
		if err != nil || now.Sub(fi.Info.ModTime()) > d {
			return false
		}
	}

	if r.Match.MimeType != "" {
		detected := detectMIME(fi.Path)
		if !strings.HasPrefix(detected, r.Match.MimeType) {
			return false
		}
	}

	// Content matching — most expensive, runs last after all cheap checks pass
	if r.Match.Content != "" || r.Match.ContentRegex != "" {
		if !matchContent(fi.Path, r.Match) {
			return false
		}
	}

	return true
}

func matchContent(path string, m Match) bool {
	maxBytes := m.ContentBytes
	if maxBytes <= 0 {
		maxBytes = 65536 // 64KB default
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, _ := f.Read(buf)
	if n == 0 {
		return false
	}
	content := string(buf[:n])

	if m.Content != "" {
		if !strings.Contains(strings.ToLower(content), strings.ToLower(m.Content)) {
			return false
		}
	}

	if m.ContentRegex != "" {
		re, err := regexp.Compile(m.ContentRegex)
		if err != nil {
			return false
		}
		if !re.MatchString(content) {
			return false
		}
	}

	return true
}

// detectMIME returns the MIME type of a file using extension-based lookup
// first, falling back to content sniffing.
func detectMIME(path string) string {
	// Try extension first (fast)
	ext := filepath.Ext(path)
	if ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}

	// Fall back to content sniffing (reads first 512 bytes)
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

// FirstMatch returns the highest-priority rule that matches the given file.
// Rules are stable-sorted by priority (descending) so that declaration order
// is preserved as a tiebreaker. Per-directory rules should precede global
// rules in the input slice to win at equal priority.
// FirstMatch returns the highest-priority rule that matches the given file.
// Deprecated: use FindMatches for continue support.
func FirstMatch(rules []Rule, fi FileInfo) *Rule {
	matches := FindMatches(rules, fi)
	if len(matches) > 0 {
		return matches[0]
	}
	return nil
}

// FindMatches returns all rules that match the given file, respecting priority
// ordering and the continue flag. Rules are stable-sorted by priority
// (descending). Matching stops at the first rule that does NOT have
// continue: true. This preserves first-match-wins as the default while
// allowing explicit fall-through.
func FindMatches(rules []Rule, fi FileInfo) []*Rule {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	slices.SortStableFunc(sorted, func(a, b Rule) int {
		return b.Priority - a.Priority // descending
	})

	var matched []*Rule
	for i := range sorted {
		if sorted[i].Matches(fi) {
			matched = append(matched, &sorted[i])
			if !sorted[i].Continue {
				break
			}
		}
	}
	return matched
}

// ParseSize parses a human-readable size string like "500MB" or "1GB" into bytes.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"B", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size %q: %w", s, err)
			}
			return int64(n * float64(m.mult)), nil
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	return n, nil
}

// ParseAge parses a human-readable age string like "30d" or "2h" into a duration.
func ParseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	units := []struct {
		suffix string
		dur    time.Duration
	}{
		{"d", 24 * time.Hour},
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSuffix(s, u.suffix)
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid age %q: %w", s, err)
			}
			return time.Duration(n * float64(u.dur)), nil
		}
	}

	return 0, fmt.Errorf("invalid age %q: missing unit (d, h, m, s)", s)
}
