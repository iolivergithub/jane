//go:build !windows

package uefi

import "os"

// readEventLog reads the UEFI (TCG) event log from securityfs, or from the
// requested file in unsafe mode.
func readEventLog(requested string) ([]byte, error) {
	return os.ReadFile(GetEventLogLocation(requested))
}
