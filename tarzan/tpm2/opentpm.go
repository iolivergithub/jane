package tpm2

import (
	"fmt"

	utilities "ta10/common"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/iolivergithub/jane/lib/tpmopen"
)

// OpenTPM opens the TPM named by the caller's tpm2/device parameter, as jp
// does (lib/tpmopen): a device such as /dev/tpmrm0, a simulator on a Unix
// socket (unix:///path, or a path that is a socket), or TPM Base Services on
// Windows, where the path is ignored. A simulator over TCP (tcp://host:port)
// makes tarzan connect to an address the caller chose, so it is accepted
// only in unsafe mode (-unsafe), like caller-chosen log files.
func OpenTPM(dev string) (transport.TPMCloser, error) {
	if tpmopen.IsNetwork(dev) && !utilities.IsUnsafe() {
		return nil, fmt.Errorf("TPM device %q is a network address; tarzan accepts tcp:// TPM simulators only when started with -unsafe", dev)
	}
	t, desc, err := tpmopen.Open(dev)
	if err != nil {
		return nil, err
	}
	fmt.Printf("TPM device >>> %v <<< opened: %v\n", dev, desc)
	return t, nil
}
