package tpm2

import (
	"strings"
	"testing"
)

// A tcp:// simulator is a caller-chosen network address: refused unless
// tarzan runs with -unsafe.
func TestOpenTPMRefusesNetworkWhenSafe(t *testing.T) {
	_, err := OpenTPM("tcp://127.0.0.1:2321")
	if err == nil || !strings.Contains(err.Error(), "-unsafe") {
		t.Errorf("OpenTPM(tcp://...) in safe mode: err = %v, want a refusal naming -unsafe", err)
	}
}
