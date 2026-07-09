//go:build linux || freebsd || openbsd || netbsd

package rule

import (
	"golang.org/x/sys/unix"
)

// getFileOrigins attempts to retrieve the source URL(s) of a downloaded file.
// On Linux/BSD, this reads the user.xdg.origin.url and user.xdg.referrer.url
// extended attributes (used by Chromium and some other downloaders).
func getFileOrigins(path string) []string {
	var origins []string
	
	buf := make([]byte, 4096)
	
	sz, err := unix.Getxattr(path, "user.xdg.origin.url", buf)
	if err == nil && sz > 0 {
		origins = append(origins, string(buf[:sz]))
	}
	
	sz, err = unix.Getxattr(path, "user.xdg.referrer.url", buf)
	if err == nil && sz > 0 {
		origins = append(origins, string(buf[:sz]))
	}
	
	return origins
}
