//go:build windows

package tpm2

import (
	"fmt"
	"github.com/google/go-tpm/legacy/tpm2"
	"io"
)

func OpenTPM(path string) (io.ReadWriteCloser, error) {
	fmt.Sprintf("TPM Device path >>> %v <<< passed as parameter. This is a Windows build and will be ignored.")
	rwc, err := tpm2.OpenTPM()

	return rwc, err
}
