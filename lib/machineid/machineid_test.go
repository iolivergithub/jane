package machineid

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFirstLineOf(t *testing.T) {
	d := t.TempDir()
	empty := filepath.Join(d, "empty")
	id := filepath.Join(d, "id")
	os.WriteFile(empty, []byte("\n"), 0o644)
	os.WriteFile(id, []byte("  0123abcd \nsecond line\n"), 0o644)

	got, err := firstLineOf(filepath.Join(d, "missing"), empty, id)
	if err != nil || got != "0123abcd" {
		t.Errorf("got %q, %v; want the first non-empty first line", got, err)
	}
	if _, err := firstLineOf(filepath.Join(d, "missing"), empty); err == nil {
		t.Error("no ID found but no error")
	}
}

// On Linux the ID falls back to the dbus file, which tarzan did not read
// before lib/machineid, so jp and tarzan disagreed on hosts without
// /etc/machine-id.
func TestGetLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	want, err := firstLineOf("/etc/machine-id", "/var/lib/dbus/machine-id")
	if err != nil {
		t.Skip("this host has no machine ID")
	}
	if got, err := Get(); err != nil || got != want {
		t.Errorf("Get() = %q, %v; want %q", got, err, want)
	}
}
