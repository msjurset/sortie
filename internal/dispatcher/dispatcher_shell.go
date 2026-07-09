package dispatcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

// lookPathOrError checks that an external tool is available on PATH.
func lookPathOrError(tool, actionType string) error {
	if _, err := exec.LookPath(tool); err != nil {
		return fmt.Errorf("tool %q required for %s action: %w", tool, actionType, err)
	}
	return nil
}

func doExec(fi rule.FileInfo, action rule.Action, captures map[string]string) error {
	cmd, err := rule.ExpandString(action.Command, fi, captures)
	if err != nil {
		return fmt.Errorf("expanding command template: %w", err)
	}

	out, err := runShellCommand(cmd)
	if err != nil {
		return fmt.Errorf("exec %q: %s: %w", cmd, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// doNotify sends a notification. If the message field starts with http:// or
// https://, it sends an HTTP POST with file metadata as JSON. Otherwise it
// delegates to the platform-specific notifyDesktop.
func doNotify(fi rule.FileInfo, action rule.Action, captures map[string]string) error {
	title, err := rule.ExpandString(action.Title, fi, captures)
	if err != nil {
		return fmt.Errorf("expanding title template: %w", err)
	}

	message, err := rule.ExpandString(action.Message, fi, captures)
	if err != nil {
		return fmt.Errorf("expanding message template: %w", err)
	}

	link, err := rule.ExpandString(action.Link, fi, captures)
	if err != nil {
		return fmt.Errorf("expanding link template: %w", err)
	}

	if title == "" {
		title = "sortie"
	}

	if strings.HasPrefix(message, "http://") || strings.HasPrefix(message, "https://") {
		return notifyWebhook(message, title, link, fi)
	}

	return notifyDesktop(title, message, link)
}

func notifyWebhook(url, title, link string, fi rule.FileInfo) error {
	payload := map[string]string{
		"title": title,
		"file":  fi.Info.Name(),
		"path":  fi.Path,
		"size":  fmt.Sprintf("%d", fi.Info.Size()),
	}
	if link != "" {
		payload["link"] = link
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
