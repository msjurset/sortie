package dispatcher

import (
	"fmt"
	"os/exec"
	"strings"
)

func notifyDesktop(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
