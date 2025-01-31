//go:build !windows

package tpm2

import (
	_ "encoding/base64"
	_ "encoding/hex"
	"fmt"
	"net/http"
	_ "strconv"
	_ "strings"

	"github.com/labstack/echo/v4"

	// this needs to be updated
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

func openTPM(dev string) (transport.TPMCloser, error) {
	tpm, err := linuxtpm.Open(dev)

	return tpm, err
}

func NewPCRs(c echo.Context) error {
	fmt.Println("NEW tpm2 pcrs called")

	tpm, err := openTPM("/dev/tpmrm0")
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open tpm with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	fmt.Printf("TPM device open by linuxtranport is %v\n", tpm)

	//DebugPCR := uint(16)
	s1 := tpm2.TPMSPCRSelection{Hash: tpm2.TPMAlgSHA1, PCRSelect: tpm2.PCClientCompatible.PCRs(0, 1, 2, 3)}
	//s2 := tpm2.TPMSPCRSelection{Hash: 0x000B, PCRSelect: []uint8{4, 5, 11, 16, 23}}

	pcrselections := []tpm2.TPMSPCRSelection{s1}
	selection := tpm2.TPMLPCRSelection{PCRSelections: pcrselections}
	fmt.Printf("PCR selection is %v\n", selection)

	pcrreadresponse, err := tpm2.PCRRead{PCRSelectionIn: selection}.Execute(tpm)
	fmt.Printf("PCR pcrreadresponse is %w, %v\n", err, pcrreadresponse)

	//tpm2.PCRSelections{[]tpm2.PCRSelection{Hash: "sha256", PCRSelect: []byte{0, 1, 2, 3}}}

	return c.JSON(http.StatusOK, "fred")
}
