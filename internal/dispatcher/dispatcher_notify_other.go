//go:build !darwin && !windows

package dispatcher

import (
	"fmt"
	"html"
	"os/exec"
	"strings"
)

func notifyDesktop(title, message, link string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("notify-send not found; install libnotify-bin for desktop notifications")
	}
	args := []string{title, message}
	if link != "" {
		args = []string{title, fmt.Sprintf(`<a href=%q>%s</a>`, link, html.EscapeString(message))}
	}
	out, err := exec.Command("notify-send", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify-send: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
