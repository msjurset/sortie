//go:build !darwin && !windows

package dispatcher

import (
	"fmt"
	"os/exec"
	"strings"
)

func notifyDesktop(title, message string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("notify-send not found; install libnotify-bin for desktop notifications")
	}
	out, err := exec.Command("notify-send", title, message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify-send: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
