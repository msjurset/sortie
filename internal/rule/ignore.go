package rule

import (
	"path/filepath"
	"strings"
)

// ShouldIgnore checks if a file should be skipped based on ignore patterns.
// Patterns from global and local lists are evaluated in order (global first,
// then local). The last matching pattern wins, like .gitignore.
//
// Pattern syntax:
//   - "*.tmp"     — glob match on filename (filepath.Match syntax)
//   - ".DS_Store" — exact filename match
//   - "!*.log"    — negation: file is NOT ignored even if a prior pattern matched
func ShouldIgnore(global, local []string, fi FileInfo) bool {
	name := fi.Info.Name()
	ignored := false

	// Evaluate global patterns first, then local (local wins on conflict)
	for _, pattern := range append(global, local...) {
		if pattern == "" {
			continue
		}

		negate := false
		if strings.HasPrefix(pattern, "!") {
			negate = true
			pattern = pattern[1:]
		}

		if matchIgnorePattern(pattern, name) {
			ignored = !negate
		}
	}

	return ignored
}

func matchIgnorePattern(pattern, name string) bool {
	// Try glob match first
	if matched, err := filepath.Match(pattern, name); err == nil && matched {
		return true
	}

	// Exact match (for patterns like ".DS_Store" or "Thumbs.db")
	if pattern == name {
		return true
	}

	// Case-insensitive exact match for common system files
	if strings.EqualFold(pattern, name) {
		return true
	}

	return false
}
