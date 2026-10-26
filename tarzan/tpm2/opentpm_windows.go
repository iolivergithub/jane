//go:build windows

package tpm2

import (
	"fmt"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/windowstpm"
)

// OpenTPM opens the TPM through Windows TPM Base Services (TBS), as jp does
// (#122). A device path such as /dev/tpmrm0 has no meaning on Windows and is
// ignored. Whether Windows lets each TPM command through is up to Windows:
// TBS only passes commands on its allow list, and Windows keeps the owner
// authorisation itself.
func OpenTPM(dev string) (transport.TPMCloser, error) {
	fmt.Printf("TPM Device >>> %v <<< passed as parameter. This is a Windows build: using TPM Base Services, the device path is ignored\n", dev)
	return windowstpm.Open()
}
