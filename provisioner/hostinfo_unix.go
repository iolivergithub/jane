//go:build !windows

package main

import (
	"os"

	"github.com/iolivergithub/jane/lib/machineid"
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
	mid, err := machineid.Get()
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
