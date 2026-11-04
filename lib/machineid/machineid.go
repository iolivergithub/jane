// Package machineid reads the host's machine ID, the same way in jp, which
// records it on the element (host.machineid), and in tarzan, which reports it
// in sys/info claims, so the sys_machineID rules compare like with like.
//
//   - Linux: /etc/machine-id, else /var/lib/dbus/machine-id.
//   - macOS and the BSDs: those files or /etc/hostid, else the kernel's host
//     UUID (kern.hostuuid on FreeBSD and DragonFly, kern.uuid on macOS).
//   - Windows: the MachineGuid Windows assigns at installation (#122).
//   - Anything else: the files, as on the BSDs.
package machineid

import (
	"errors"
	"os"
	"strings"
)

// Get returns the machine ID, or an error if none can be found.
func Get() (string, error) {
	return get()
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
