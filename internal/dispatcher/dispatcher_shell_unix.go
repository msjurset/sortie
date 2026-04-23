//go:build !windows

package dispatcher

import "os/exec"

func runShellCommand(cmd string) ([]byte, error) {
	return exec.Command("sh", "-c", cmd).CombinedOutput()
}
