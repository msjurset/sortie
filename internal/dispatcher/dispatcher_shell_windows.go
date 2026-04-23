package dispatcher

import "os/exec"

func runShellCommand(cmd string) ([]byte, error) {
	return exec.Command("cmd", "/c", cmd).CombinedOutput()
}
