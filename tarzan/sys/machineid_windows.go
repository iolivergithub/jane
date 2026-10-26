//go:build windows

package sys

import (
	"golang.org/x/sys/windows/registry"
)

// machineID is the MachineGuid Windows assigns at installation, the value jp
// uses as the element's machine ID on Windows (#122), or "" if it can't be
// read.
func machineID() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return v
}
