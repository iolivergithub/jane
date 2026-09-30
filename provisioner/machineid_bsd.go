//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"strings"

	"golang.org/x/sys/unix"
)

// machineID prefers a machine-id file (set up by some BSD installs), then the
// kernel's host UUID: kern.hostuuid on FreeBSD/DragonFly, kern.uuid on macOS.
func machineID() (string, error) {
	id, err := firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id", "/etc/hostid")
	if err == nil {
		return id, nil
	}
	for _, name := range []string{"kern.hostuuid", "kern.uuid"} {
		if v, serr := unix.Sysctl(name); serr == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	return "", err
}
