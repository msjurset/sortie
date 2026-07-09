//go:build darwin

package rule

import (
	"os/exec"
	"regexp"
)

// Extract URLs printed inside quotes by mdls
var mdlsUrlRegex = regexp.MustCompile(`"([^"]+)"`)

// getFileOrigins attempts to retrieve the source URL(s) of a downloaded file.
func getFileOrigins(path string) []string {
	out, err := exec.Command("mdls", "-name", "kMDItemWhereFroms", path).CombinedOutput()
	if err != nil {
		return nil
	}
	
	// If the attribute doesn't exist, mdls outputs:
	// kMDItemWhereFroms = (null)
	
	matches := mdlsUrlRegex.FindAllStringSubmatch(string(out), -1)
	var origins []string
	for _, m := range matches {
		origins = append(origins, m[1])
	}
	return origins
}
