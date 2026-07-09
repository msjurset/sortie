package dispatcher

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func notifyDesktop(title, message, link string) error {
	t := escapePSSingleQuote(title)
	m := escapePSSingleQuote(message)

	var script string
	if link != "" {
		l := escapePSSingleQuote(link)
		script = fmt.Sprintf(
			`if (Get-Module -ListAvailable -Name BurntToast) { Import-Module BurntToast; $btn = New-BTButton -Content 'Open' -Arguments '%s'; New-BurntToastNotification -Text '%s','%s' -Button $btn } else { exit 2 }`,
			l, t, m,
		)
	} else {
		script = fmt.Sprintf(
			`if (Get-Module -ListAvailable -Name BurntToast) { Import-Module BurntToast; New-BurntToastNotification -Text '%s','%s' } else { exit 2 }`,
			t, m,
		)
	}

	err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script).Run()
	if err == nil {
		return nil
	}

	// Fallback: write to stderr so the notification is still visible in the
	// daemon log. Install BurntToast ("Install-Module BurntToast") for native
	// Windows 10+ toast notifications.
	if link != "" {
		fmt.Fprintf(os.Stderr, "[sortie notify] %s: %s (%s)\n", title, message, link)
	} else {
		fmt.Fprintf(os.Stderr, "[sortie notify] %s: %s\n", title, message)
	}
	return nil
}

func escapePSSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
