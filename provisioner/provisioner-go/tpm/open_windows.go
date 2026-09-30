//go:build windows

package tpm

import (
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/windowstpm"
)

// openPlatform uses TPM Base Services. A device path (such as the
// /dev/tpmrm0 of a shared provisioning file) has no meaning on Windows and is
// ignored.
func openPlatform(device string) (transport.TPMCloser, string, error) {
	t, err := windowstpm.Open()
	if err != nil {
		return nil, "", err
	}
	desc := "Windows TPM Base Services"
	if device != "" {
		desc += " (device " + device + " ignored)"
	}
	return t, desc, nil
}
