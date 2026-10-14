package tpm2

import (
	"testing"

	"github.com/google/go-tpm/tpm2"
)

// A bank that is allocated but not populated returns no digests; this must
// not panic (#15).
func TestFirstDigestHexEmpty(t *testing.T) {
	if v, ok := firstDigestHex(nil); ok || v != "" {
		t.Errorf("firstDigestHex(nil) = %q, %v; want \"\", false", v, ok)
	}
	if v, ok := firstDigestHex([]tpm2.TPM2BDigest{}); ok || v != "" {
		t.Errorf("firstDigestHex(empty) = %q, %v; want \"\", false", v, ok)
	}
}

func TestFirstDigestHex(t *testing.T) {
	d := []tpm2.TPM2BDigest{{Buffer: []byte{0xde, 0xad, 0xbe, 0xef}}}
	if v, ok := firstDigestHex(d); !ok || v != "deadbeef" {
		t.Errorf("firstDigestHex = %q, %v; want \"deadbeef\", true", v, ok)
	}
}

func TestTPMDeviceParam(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]interface{}
		want   string
	}{
		{"given", map[string]interface{}{"tpm2/device": "/dev/tpmrm0"}, "/dev/tpmrm0"},
		{"missing", map[string]interface{}{}, defaultTPMDevice},
		{"nil map", nil, defaultTPMDevice},
		{"empty", map[string]interface{}{"tpm2/device": ""}, defaultTPMDevice},
		{"not a string", map[string]interface{}{"tpm2/device": 0}, defaultTPMDevice},
	}
	for _, c := range cases {
		if got := tpmDeviceParam(c.params); got != c.want {
			t.Errorf("%s: tpmDeviceParam = %q, want %q", c.name, got, c.want)
		}
	}
}
