//go:build !darwin

package dispatcher

import (
	"fmt"
	"runtime"

	"github.com/msjurset/sortie/internal/rule"
)

func doOpen(fi rule.FileInfo, action rule.Action) error {
	return fmt.Errorf("open action is only supported on macOS (current platform: %s)", runtime.GOOS)
}

func doUnquarantine(path string) error {
	return nil
}
