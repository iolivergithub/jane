//go:build !windows

package tpm

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
	"github.com/google/go-tpm/tpm2/transport/linuxudstpm"
)

// DefaultDevices are tried in order when no device is given.
var DefaultDevices = []string{"/dev/tpmrm0", "/dev/tpm0"}

func openPlatform(device string) (transport.TPMCloser, string, error) {
	if device == "" {
		var errs []error
		for _, d := range DefaultDevices {
			t, err := linuxtpm.Open(d)
			if err == nil {
				return t, "TPM device " + d, nil
			}
			errs = append(errs, err)
		}
		return nil, "", fmt.Errorf("no TPM device found: %w", errors.Join(errs...))
	}

	if sock, ok := strings.CutPrefix(device, "unix://"); ok {
		return openSocket(sock)
	}
	if strings.Contains(device, "://") {
		return nil, "", fmt.Errorf("unsupported TPM device %q", device)
	}

	fi, err := os.Stat(device)
	if err != nil {
		return nil, "", err
	}
	if fi.Mode()&os.ModeSocket != 0 {
		return openSocket(device)
	}
	t, err := linuxtpm.Open(device)
	if err != nil {
		return nil, "", err
	}
	return t, "TPM device " + device, nil
}

func openSocket(path string) (transport.TPMCloser, string, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, "", err
	}
	t, err := linuxudstpm.Open(path)
	if err != nil {
		return nil, "", err
	}
	return t, "TPM socket " + path, nil
}
