//go:build windows

package rule

import (
	"bufio"
	"os"
	"strings"
)

// getFileOrigins attempts to retrieve the source URL(s) of a downloaded file.
// On Windows, browsers write origin info into the Zone.Identifier Alternate Data Stream (ADS).
func getFileOrigins(path string) []string {
	f, err := os.Open(path + ":Zone.Identifier")
	if err != nil {
		return nil
	}
	defer f.Close()
	
	var origins []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "HostUrl=") {
			origins = append(origins, strings.TrimPrefix(line, "HostUrl="))
		} else if strings.HasPrefix(line, "ReferrerUrl=") {
			origins = append(origins, strings.TrimPrefix(line, "ReferrerUrl="))
		}
	}
	return origins
}
