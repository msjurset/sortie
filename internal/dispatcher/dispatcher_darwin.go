package dispatcher

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

func doOpen(fi rule.FileInfo, action rule.Action) error {
	args := []string{fi.Path}
	if action.App != "" {
		args = []string{"-a", action.App, fi.Path}
	}

	out, err := exec.Command("open", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func doUnquarantine(path string) error {
	out, err := exec.Command("xattr", "-l", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reading xattrs: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if !strings.Contains(string(out), "com.apple.quarantine") {
		return nil
	}

	out, err = exec.Command("xattr", "-d", "com.apple.quarantine", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing quarantine: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
