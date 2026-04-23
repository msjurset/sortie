package dispatcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

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

	if info, statErr := os.Stat(dest); statErr == nil && !info.IsDir() {
		destHash, hashErr := hashFile(dest)
		if hashErr == nil && srcHash == destHash {
			if onDuplicate == "delete" {
				return "delete", os.Remove(src)
			}
			return "skip", nil
		}
	}

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
