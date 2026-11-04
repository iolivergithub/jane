package tpmopen

import (
	"runtime"
	"testing"
)

func TestOpenErrors(t *testing.T) {
	bad := []string{"tcp://no-port", "tcp://localhost:notaport", "tcp://localhost:0"}
	if runtime.GOOS != "windows" {
		bad = append(bad, "http://x", "/nonexistent/tpm", "unix:///nonexistent/sock")
	}
	for _, d := range bad {
		if tp, _, err := Open(d); err == nil {
			tp.Close()
			t.Errorf("Open(%q) succeeded", d)
		}
	}
}

func TestIsNetwork(t *testing.T) {
	for d, want := range map[string]bool{
		"tcp://localhost:2321":   true,
		" tcp://10.0.0.1:2321":   true,
		"/dev/tpmrm0":            false,
		"unix:///run/swtpm.sock": false,
		"":                       false,
	} {
		if got := IsNetwork(d); got != want {
			t.Errorf("IsNetwork(%q) = %v", d, got)
		}
	}
}
