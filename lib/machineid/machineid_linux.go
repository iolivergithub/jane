//go:build linux

package machineid

func get() (string, error) {
	return firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id")
}
