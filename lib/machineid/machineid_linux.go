//go:build linux

package machineid

import "errors"

// get reads the systemd machine ID, else the D-Bus one, which non-systemd
// distributions (Alpine, Void, Devuan, Gentoo with OpenRC) may have instead.
func get() (string, error) {
	id, err := firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id")
	if err != nil {
		return "", errors.Join(err, errors.New("create one with systemd-machine-id-setup, or dbus-uuidgen --ensure on systems without systemd"))
	}
	return id, nil
}
