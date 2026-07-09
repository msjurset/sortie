package dispatcher

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

var terminalNotifierWarnOnce sync.Once

// notifyDesktop sends a desktop notification on macOS. When link is empty it
// uses osascript's `display notification`. When link is set, it requires
// terminal-notifier (https://github.com/julienXX/terminal-notifier) for the
// banner to be clickable; if terminal-notifier is missing, it falls back to a
// plain osascript notification and logs a one-time warning.
func notifyDesktop(title, message, link string) error {
	if link != "" {
		if path, err := exec.LookPath("terminal-notifier"); err == nil {
			args := []string{"-title", title, "-message", message, "-open", link}
			out, err := exec.Command(path, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("terminal-notifier: %s: %w", strings.TrimSpace(string(out)), err)
			}
			return nil
		}
		terminalNotifierWarnOnce.Do(func() {
			log.Printf("notify: link field requires terminal-notifier on macOS; install with `brew install terminal-notifier` to make notifications clickable. Falling back to plain notification.")
		})
	}

	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
