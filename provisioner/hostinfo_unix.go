//go:build !windows

package main

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// collectHostInfo gathers the host section of the element. OS is built from
// uname as sysname-release-machine, close to Python's platform.platform()
// but without the libc suffix.
func collectHostInfo() (HostInfo, error) {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return HostInfo{}, err
	}
	h, err := os.Hostname()
	if err != nil {
		return HostInfo{}, err
	}
	mid, err := machineID()
	if err != nil {
		return HostInfo{}, err
	}
	machine := unix.ByteSliceToString(u.Machine[:])
	return HostInfo{
		OS:        unix.ByteSliceToString(u.Sysname[:]) + "-" + unix.ByteSliceToString(u.Release[:]) + "-" + machine,
		Arch:      machine,
		Hostname:  h,
		MachineID: mid,
	}, nil
}

// firstLineOf returns the first non-empty first line among files.
func firstLineOf(files ...string) (string, error) {
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		line, _, _ := strings.Cut(string(b), "\n")
		if line = strings.TrimSpace(line); line != "" {
			return line, nil
		}
	}
	return "", errors.New("no machine ID found in " + strings.Join(files, ", "))
}
