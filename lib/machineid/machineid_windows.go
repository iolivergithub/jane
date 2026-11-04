//go:build windows

package machineid

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// get returns the MachineGuid Windows assigns at installation.
func get() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("opening Cryptography key: %w", err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("reading MachineGuid: %w", err)
	}
	return v, nil
}
