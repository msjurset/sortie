package dispatcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/msjurset/sortie/internal/rule"
)

// doOpen opens a file with the default application or a specified app.
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

// doDeduplicate checks if an identical file (by SHA-256) already exists at dest.
// Returns the outcome: "moved" if no duplicate and file was moved, "skip" if
// duplicate found and source left in place, "delete" if duplicate found and
// source removed.
func doDeduplicate(src, dest, onDuplicate string) (string, error) {
	if dest == "" {
		return "", fmt.Errorf("deduplicate action requires dest field")
	}

	srcHash, err := hashFile(src)
	if err != nil {
		return "", fmt.Errorf("hashing source: %w", err)
	}

	// Check if dest exists and has the same hash
	if info, statErr := os.Stat(dest); statErr == nil && !info.IsDir() {
		destHash, hashErr := hashFile(dest)
		if hashErr == nil && srcHash == destHash {
			// Duplicate found
			if onDuplicate == "delete" {
				return "delete", os.Remove(src)
			}
			return "skip", nil
		}
	}

	// Not a duplicate — move the file
	if err := doMove(src, dest); err != nil {
		return "", err
	}
	return "moved", nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// doUnquarantine removes the com.apple.quarantine extended attribute from a file.
func doUnquarantine(path string) error {
	// Check if the xattr exists first
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
