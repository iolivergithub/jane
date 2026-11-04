//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package machineid

func get() (string, error) {
	return firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id", "/etc/hostid")
}
