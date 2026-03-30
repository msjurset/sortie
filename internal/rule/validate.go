package rule

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Severity indicates how serious a validation finding is.
type Severity int

const (
	SeverityError   Severity = iota // config will not work correctly
	SeverityWarning                 // config may cause unexpected behavior
)

func (s Severity) String() string {
	if s == SeverityError {
		return "error"
	}
	return "warning"
}

// Finding represents a single validation issue.
type Finding struct {
	Severity Severity
	Rule     string // rule name
	Message  string
}

func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s: %s", f.Severity, f.Rule, f.Message)
}

// WatchedDir represents a directory being monitored with its recursive setting.
type WatchedDir struct {
	Path      string
	Recursive bool
}

// ValidateRules checks a set of rules for configuration errors and potential
// problems. watchedDirs is the list of directories being monitored (used to
// detect potential infinite loops).
func ValidateRules(rules []Rule, watchedDirs []WatchedDir) []Finding {
	var findings []Finding

	for _, r := range rules {
		findings = append(findings, validateRule(r, watchedDirs)...)
	}

	return findings
}

func validateRule(r Rule, watchedDirs []WatchedDir) []Finding {
	var findings []Finding

	actions := r.ResolvedActions()

	if len(actions) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     r.Name,
			Message:  "no actions defined (need action: or actions:)",
		})
		return findings
	}

	// Validate match conditions
	findings = append(findings, ValidateMatch(r.Name, r.Match)...)

	// Validate that {{.Match.}} templates have corresponding capture groups
	findings = append(findings, validateCaptureRefs(r)...)

	// Validate cooldown
	if r.Cooldown != "" {
		if _, err := ParseAge(r.Cooldown); err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     r.Name,
				Message:  fmt.Sprintf("invalid cooldown %q: %v", r.Cooldown, err),
			})
		}
	}

	// Validate each action individually
	for i, a := range actions {
		prefix := ""
		if len(actions) > 1 {
			prefix = fmt.Sprintf("action[%d] ", i+1)
		}
		findings = append(findings, validateAction(r.Name, prefix, a)...)
	}

	// Validate chain combinations
	if len(actions) > 1 {
		findings = append(findings, validateChain(r.Name, actions)...)
	}

	// Check for infinite loop risk (dest inside a watched directory)
	findings = append(findings, validateLoopRisk(r.Name, actions, watchedDirs)...)

	return findings
}

// validateAction checks a single action for missing required fields and
// invalid values.
func validateAction(ruleName, prefix string, a Action) []Finding {
	var findings []Finding
	f := func(sev Severity, msg string) {
		findings = append(findings, Finding{sev, ruleName, prefix + msg})
	}

	switch a.Type {
	case "":
		f(SeverityError, "action type is empty")
		return findings

	case ActionMove, ActionCopy, ActionRename, ActionSymlink:
		if a.Dest == "" {
			f(SeverityError, fmt.Sprintf("%s action requires dest field", a.Type))
		}

	case ActionCompress:
		// dest is optional (defaults to src + .gz)

	case ActionDelete:
		// no required fields

	case ActionExtract:
		if a.Dest == "" {
			f(SeverityError, "extract action requires dest field")
		}

	case ActionChmod:
		if a.Mode == "" {
			f(SeverityError, "chmod action requires mode field")
		} else if !isValidOctalMode(a.Mode) {
			f(SeverityError, fmt.Sprintf("chmod mode %q is not a valid octal permission (e.g., 0644, 755)", a.Mode))
		}

	case ActionChecksum:
		if a.Algorithm != "" && a.Algorithm != "sha256" && a.Algorithm != "md5" && a.Algorithm != "sha1" {
			f(SeverityError, fmt.Sprintf("unsupported checksum algorithm %q (use sha256, md5, or sha1)", a.Algorithm))
		}

	case ActionExec:
		if a.Command == "" {
			f(SeverityError, "exec action requires command field")
		}

	case ActionNotify:
		if a.Message == "" && a.Title == "" {
			f(SeverityWarning, "notify action has no title or message")
		}

	case ActionConvert:
		if a.Tool == "" {
			f(SeverityError, "convert action requires tool field (e.g., ffmpeg, convert, pandoc)")
		}
		if a.Args == "" {
			f(SeverityWarning, "convert action has no args field")
		}
		if a.Dest == "" {
			f(SeverityWarning, "convert action has no dest field")
		}

	case ActionResize:
		if a.Width == 0 && a.Height == 0 && a.Percentage == 0 {
			f(SeverityError, "resize action requires width, height, or percentage")
		}
		if a.Dest == "" {
			f(SeverityError, "resize action requires dest field")
		}

	case ActionWatermark:
		if a.Overlay == "" {
			f(SeverityError, "watermark action requires overlay field")
		}
		if a.Dest == "" {
			f(SeverityError, "watermark action requires dest field")
		}

	case ActionOCR:
		// all fields optional (defaults: tesseract, eng, sidecar .txt)

	case ActionEncrypt:
		if a.Recipient == "" {
			f(SeverityError, "encrypt action requires recipient field")
		}

	case ActionDecrypt:
		// key is optional (gpg can use default keyring)

	case ActionUpload:
		if a.Remote == "" {
			f(SeverityError, "upload action requires remote field")
		}

	case ActionTag:
		if len(a.Tags) == 0 {
			f(SeverityError, "tag action requires tags field")
		}

	case ActionOpen:
		// all fields optional

	case ActionDeduplicate:
		if a.Dest == "" {
			f(SeverityError, "deduplicate action requires dest field")
		}
		if a.OnDuplicate != "" && a.OnDuplicate != "skip" && a.OnDuplicate != "delete" {
			f(SeverityError, fmt.Sprintf("on_duplicate must be \"skip\" or \"delete\", got %q", a.OnDuplicate))
		}

	case ActionUnquarantine:
		// no required fields

	default:
		f(SeverityError, fmt.Sprintf("unknown action type %q", a.Type))
	}

	return findings
}

// validateChain checks for problematic action combinations in a chain.
func validateChain(ruleName string, actions []Action) []Finding {
	var findings []Finding
	f := func(sev Severity, msg string) {
		findings = append(findings, Finding{sev, ruleName, msg})
	}

	for i, a := range actions {
		isLast := i == len(actions)-1

		// delete should be the last action — file is moved to trash
		if a.Type == ActionDelete && !isLast {
			f(SeverityError, fmt.Sprintf("action[%d] delete must be the last action in a chain (file is moved to trash)", i+1))
		}

		// compress removes the original — subsequent actions won't find it
		if a.Type == ActionCompress && !isLast {
			f(SeverityWarning, fmt.Sprintf("action[%d] compress removes the original file; subsequent actions may fail", i+1))
		}

		// deduplicate outcome is conditional — subsequent actions may not make sense
		if a.Type == ActionDeduplicate && !isLast {
			f(SeverityWarning, fmt.Sprintf("action[%d] deduplicate has conditional outcome (skip/move/delete); subsequent actions may not execute as expected", i+1))
		}

		// open is async — subsequent actions race with the launched app
		if a.Type == ActionOpen && !isLast {
			f(SeverityWarning, fmt.Sprintf("action[%d] open launches an app asynchronously; subsequent actions may race with it", i+1))
		}

		// Two moves/renames in a row is likely a mistake
		if i > 0 && (a.Type == ActionMove || a.Type == ActionRename) {
			prev := actions[i-1]
			if prev.Type == ActionMove || prev.Type == ActionRename {
				f(SeverityWarning, fmt.Sprintf("action[%d] consecutive %s after %s is usually a mistake", i+1, a.Type, prev.Type))
			}
		}
	}

	return findings
}

// validateLoopRisk checks if any action's destination falls inside a watched
// directory, which could cause an infinite dispatch loop.
//
// When a watched directory is not recursive, only destinations in the exact
// same directory trigger a warning (subdirectories are safe). When recursive,
// any subdirectory also triggers a warning.
func validateLoopRisk(ruleName string, actions []Action, watchedDirs []WatchedDir) []Finding {
	var findings []Finding

	for i, a := range actions {
		dest := a.Dest
		if dest == "" {
			continue
		}

		// Strip template variables for directory checking — we can only
		// check the static prefix. e.g., ~/Documents/{{.Year}}/ has a
		// static prefix of ~/Documents/.
		staticDir := templateStaticPrefix(dest)
		if staticDir == "" {
			continue
		}

		for _, wd := range watchedDirs {
			var risk bool
			if wd.Recursive {
				// Any file under the watched dir is at risk
				risk = isSubpath(staticDir, wd.Path)
			} else {
				// Only files placed directly in the watched dir are at risk
				risk = filepath.Clean(staticDir) == filepath.Clean(wd.Path)
			}

			if risk {
				prefix := ""
				if len(actions) > 1 {
					prefix = fmt.Sprintf("action[%d] ", i+1)
				}
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Rule:     ruleName,
					Message:  fmt.Sprintf("%s%s dest %q is inside watched directory %q — risk of infinite dispatch loop", prefix, a.Type, dest, wd.Path),
				})
			}
		}
	}

	return findings
}

// templateStaticPrefix returns the directory where the dispatched file will
// actually land, considering ExpandTemplate's behavior: if the final path
// segment has no extension, it's treated as a directory and the original
// filename is appended. Template variables are stripped to get the static
// prefix.
//
// Examples:
//
//	"~/Documents/file.txt"                -> "~/Documents"
//	"~/Documents/PDFs"                    -> "~/Documents/PDFs"  (directory dest)
//	"~/Documents/{{.Year}}/{{.Name}}.pdf" -> "~/Documents"
//	"{{.Name}}"                           -> ""
func templateStaticPrefix(path string) string {
	idx := strings.Index(path, "{{")

	if idx == -1 {
		// No templates — check if final segment looks like a file or dir
		base := filepath.Base(path)
		if strings.Contains(base, ".") {
			// Has extension — it's a file, return parent dir
			return filepath.Dir(path)
		}
		// No extension — ExpandTemplate treats this as a directory
		return filepath.Clean(path)
	}

	if idx == 0 {
		return ""
	}

	// Has template — take the static portion before the first {{
	static := path[:idx]
	// If static ends with a separator, the directory is the static part
	if static[len(static)-1] == filepath.Separator || static[len(static)-1] == '/' {
		return filepath.Clean(static)
	}
	return filepath.Dir(static)
}

// isSubpath returns true if child is inside parent (or equal to it).
func isSubpath(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)

	if child == parent {
		return true
	}

	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// ValidateMatch checks match conditions for potential issues.
func ValidateMatch(ruleName string, m Match) []Finding {
	var findings []Finding
	if m.ContentRegex != "" {
		if _, err := regexp.Compile(m.ContentRegex); err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     ruleName,
				Message:  fmt.Sprintf("invalid content_regex %q: %v", m.ContentRegex, err),
			})
		}
	}
	if m.ContentBytes > 10*1024*1024 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     ruleName,
			Message:  fmt.Sprintf("content_bytes is %d (>10MB) — may be slow on large directories", m.ContentBytes),
		})
	}
	if (m.Content != "" || m.ContentRegex != "") && len(m.Extensions) == 0 && m.Glob == "" && m.MimeType == "" {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Rule:     ruleName,
			Message:  "content matching without extension, glob, or mime_type filter will read every file",
		})
	}
	return findings
}

// ValidateIgnorePatterns checks that ignore patterns are valid globs.
func ValidateIgnorePatterns(patterns []string, source string) []Finding {
	var findings []Finding
	for _, p := range patterns {
		pattern := p
		if strings.HasPrefix(pattern, "!") {
			pattern = pattern[1:]
		}
		if pattern == "" {
			continue
		}
		if _, err := filepath.Match(pattern, "test"); err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Rule:     source,
				Message:  fmt.Sprintf("invalid ignore pattern %q: %v", p, err),
			})
		}
	}
	return findings
}

// validateCaptureRefs checks that templates referencing {{.Match.}} have
// corresponding named capture groups in content_regex.
func validateCaptureRefs(r Rule) []Finding {
	var findings []Finding

	// Collect all template strings from actions
	var templates []string
	for _, a := range r.ResolvedActions() {
		templates = append(templates, a.Dest, a.Command, a.Title, a.Message, a.Args, a.Remote)
	}

	// Check if any template references .Match.
	usesMatch := false
	for _, t := range templates {
		if strings.Contains(t, ".Match.") {
			usesMatch = true
			break
		}
	}
	if !usesMatch {
		return nil
	}

	if r.Match.ContentRegex == "" {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     r.Name,
			Message:  "templates reference {{.Match.}} but no content_regex is defined",
		})
		return findings
	}

	re, err := regexp.Compile(r.Match.ContentRegex)
	if err != nil {
		return nil // already caught by ValidateMatch
	}

	groupNames := map[string]bool{}
	for _, name := range re.SubexpNames() {
		if name != "" {
			groupNames[name] = true
		}
	}

	if len(groupNames) == 0 {
		findings = append(findings, Finding{
			Severity: SeverityError,
			Rule:     r.Name,
			Message:  "templates reference {{.Match.}} but content_regex has no named capture groups (use (?P<name>...))",
		})
		return findings
	}

	// Check each referenced name exists as a capture group
	refRe := regexp.MustCompile(`\{\{[^}]*\.Match\.(\w+)`)
	for _, t := range templates {
		for _, m := range refRe.FindAllStringSubmatch(t, -1) {
			name := m[1]
			if !groupNames[name] {
				findings = append(findings, Finding{
					Severity: SeverityWarning,
					Rule:     r.Name,
					Message:  fmt.Sprintf("template references {{.Match.%s}} but content_regex has no group named %q", name, name),
				})
			}
		}
	}

	return findings
}

func isValidOctalMode(s string) bool {
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return len(s) >= 3 && len(s) <= 4
}
