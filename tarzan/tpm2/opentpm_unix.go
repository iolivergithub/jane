//go:build !windows

package tpm2

import (
	"fmt"
	"slices"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

var TPMDEVICES = []string{"/dev/tpm0", "/dev/tpmrm0", "/dev/tpm1", "/dev/tpmrm1"}

func OpenTPM(dev string) (transport.TPMCloser, error) {
	fmt.Printf("TPM Device >>> %v <<< passed as parameter. This is a Unix build: ", dev)

	// Check if the path is a known device, else treat it as a unix domain socket
	if slices.Contains(TPMDEVICES, dev) {
		fmt.Printf("Treating it as a device\n")
		return linuxtpm.Open(dev)
	} else {
		fmt.Printf("Treating it as a TCP Unix domain socket\n")
		return linuxtpm.Open(dev)
	}
}
