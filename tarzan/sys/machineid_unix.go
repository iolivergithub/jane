//go:build !windows

package sys

import (
	"os"
	"strings"
)

// machineID is the contents of /etc/machine-id, or "" if it can't be read.
func machineID() string {
	b, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), " \t\n")
}
