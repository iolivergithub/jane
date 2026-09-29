//go:build linux

package main

import (
	"os"
	"strings"
	"syscall"
)

// collectHostInfo gathers the host section of the element. OS is built from
// uname as sysname-release-machine, close to Python's platform.platform()
// but without the libc suffix.
func collectHostInfo() (HostInfo, error) {
	var u syscall.Utsname
	if err := syscall.Uname(&u); err != nil {
		return HostInfo{}, err
	}
	h, err := os.Hostname()
	if err != nil {
		return HostInfo{}, err
	}
	b, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return HostInfo{}, err
	}
	mid, _, _ := strings.Cut(string(b), "\n")

	machine := utsString(u.Machine[:])
	return HostInfo{
		OS:        utsString(u.Sysname[:]) + "-" + utsString(u.Release[:]) + "-" + machine,
		Arch:      machine,
		Hostname:  h,
		MachineID: strings.TrimSpace(mid),
	}, nil
}

// utsString converts a NUL-terminated utsname field; the element type is
// int8 on some architectures (amd64) and uint8 on others (arm64).
func utsString[T int8 | uint8](f []T) string {
	b := make([]byte, 0, len(f))
	for _, c := range f {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}
