package tpm

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/tcp"
)

// Open opens the TPM named by device and returns it with a description for
// logging. Accepted forms:
//
//	""                  the platform default: TPM Base Services on Windows,
//	                    otherwise /dev/tpmrm0, falling back to /dev/tpm0
//	/dev/tpmrm0         a TPM character device (not on Windows, where any
//	                    path is ignored and TBS is used, since the same
//	                    provisioning file may be shared between platforms)
//	unix:///path        a TPM simulator on a Unix domain socket, e.g. swtpm
//	                    --server type=unixio (not on Windows); a plain path
//	                    that is a socket works too
//	tcp://host:port     a TPM simulator speaking the Microsoft simulator
//	                    protocol (ms-tpm-20-ref, swtpm --server type=tcp),
//	                    with its platform port at port+1. The simulator must
//	                    already be powered on and started.
func Open(device string) (transport.TPMCloser, string, error) {
	device = strings.TrimSpace(device)
	if rest, ok := strings.CutPrefix(device, "tcp://"); ok {
		return openTCP(rest)
	}
	return openPlatform(device)
}

func openTCP(addr string) (transport.TPMCloser, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", fmt.Errorf("bad TPM simulator address %q: %w", addr, err)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p <= 0 || p >= 65535 {
		return nil, "", fmt.Errorf("bad TPM simulator port %q", port)
	}
	plat := net.JoinHostPort(host, strconv.Itoa(p+1))
	t, err := tcp.Open(tcp.Config{CommandAddress: addr, PlatformAddress: plat})
	if err != nil {
		return nil, "", err
	}
	return t, "TPM simulator at " + addr, nil
}
