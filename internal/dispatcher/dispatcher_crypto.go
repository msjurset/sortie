package dispatcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

// doEncrypt encrypts a file using age (default) or gpg.
// The source file is preserved; encrypted output goes to dest.
func doEncrypt(fi rule.FileInfo, action rule.Action, dest string) error {
	if action.Recipient == "" {
		return fmt.Errorf("encrypt action requires recipient field")
	}

	tool := action.Tool
	if tool == "" {
		tool = "age"
	}

	if err := lookPathOrError(tool, "encrypt"); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	var cmd *exec.Cmd
	switch tool {
	case "age":
		cmd = exec.Command("age", "-r", action.Recipient, "-o", dest, fi.Path)
	case "gpg":
		cmd = exec.Command("gpg", "--encrypt", "--recipient", action.Recipient, "--output", dest, fi.Path)
	default:
		cmd = exec.Command(tool, fi.Path, dest)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s encrypt: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// doDecrypt decrypts a file using age (default) or gpg.
// The source file is preserved; decrypted output goes to dest.
func doDecrypt(fi rule.FileInfo, action rule.Action, dest string) error {
	tool := action.Tool
	if tool == "" {
		tool = "age"
	}

	if err := lookPathOrError(tool, "decrypt"); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	var cmd *exec.Cmd
	switch tool {
	case "age":
		args := []string{"-d"}
		if action.Key != "" {
			args = append(args, "-i", action.Key)
		}
		args = append(args, "-o", dest, fi.Path)
		cmd = exec.Command("age", args...)
	case "gpg":
		args := []string{"--decrypt", "--output", dest}
		if action.Key != "" {
			args = append(args, "--keyring", action.Key)
		}
		args = append(args, fi.Path)
		cmd = exec.Command("gpg", args...)
	default:
		cmd = exec.Command(tool, fi.Path, dest)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s decrypt: %s: %w", tool, strings.TrimSpace(string(out)), err)
	}
	return nil
}
