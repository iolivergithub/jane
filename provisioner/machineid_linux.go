//go:build linux

package main

func machineID() (string, error) {
	return firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id")
}
