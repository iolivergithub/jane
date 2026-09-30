//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// collectHostInfo gathers the host section of the element on Windows. OS is
// Windows-<major>.<minor>.<build>, Arch is the processor architecture as
// Python's platform.machine() reports it (e.g. AMD64), and the machine ID is
// the MachineGuid Windows assigns at installation.
func collectHostInfo() (HostInfo, error) {
	h, err := os.Hostname()
	if err != nil {
		return HostInfo{}, err
	}

	crypto, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return HostInfo{}, fmt.Errorf("opening Cryptography key: %w", err)
	}
	defer crypto.Close()
	mid, _, err := crypto.GetStringValue("MachineGuid")
	if err != nil {
		return HostInfo{}, fmt.Errorf("reading MachineGuid: %w", err)
	}

	osName := "Windows"
	if cv, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE|registry.WOW64_64KEY); err == nil {
		major, _, err1 := cv.GetIntegerValue("CurrentMajorVersionNumber")
		minor, _, err2 := cv.GetIntegerValue("CurrentMinorVersionNumber")
		build, _, err3 := cv.GetStringValue("CurrentBuild")
		if err1 == nil && err2 == nil && err3 == nil {
			osName = fmt.Sprintf("Windows-%d.%d.%s", major, minor, build)
		}
		cv.Close()
	}

	arch := os.Getenv("PROCESSOR_ARCHITEW6432") // set for 32-bit processes on 64-bit Windows
	if arch == "" {
		arch = os.Getenv("PROCESSOR_ARCHITECTURE")
	}

	return HostInfo{OS: osName, Arch: arch, Hostname: h, MachineID: mid}, nil
}
