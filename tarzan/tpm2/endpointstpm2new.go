//go:build !windows

package tpm2

import (
	_ "encoding/base64"
	_ "encoding/hex"
	"fmt"
	"net/http"
	"reflect"
	_ "strconv"
	_ "strings"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

func openTPM(dev string) (transport.TPMCloser, error) {
	tpm, err := linuxtpm.Open(dev)

	return tpm, err
}

func NewPCRs(c echo.Context) error {

	//results := []tpm2.PCRReadResponse{}
	banks := make(map[string]pcrValue)

	fmt.Println("NEW tpm2 pcrs called")

	tpm, err := openTPM("/dev/tpmrm0")
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open tpm with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	fmt.Printf("TPM device open by linuxtransport is %v\n", tpm)

	for _, b := range npcrbanks {
		pcrvs := make(map[int]string)
		for i := 0; i <= 23; i++ {
			fmt.Printf("Reading back %v, pcr %v -->\n", b, i)

			s2 := tpm2.TPMSPCRSelection{Hash: b, PCRSelect: tpm2.PCClientCompatible.PCRs(uint(i))}

			pcrselections := []tpm2.TPMSPCRSelection{s2}
			selection := tpm2.TPMLPCRSelection{PCRSelections: pcrselections}
			fmt.Printf("PCR selection is %v\n", selection)

			pcrreadresponse, err := tpm2.PCRRead{PCRSelectionIn: selection}.Execute(tpm)
			pcrvalues := *pcrreadresponse
			digests := pcrvalues.PCRValues.Digests[0]
			digestsAsString := digests.Buffer

			fmt.Printf("PCR pcrreadresponse is %w, %v\n", err, pcrreadresponse)
			ashex := fmt.Sprintf("%x", digestsAsString)
			fmt.Printf("  PCRValues are %v => %v\n", reflect.TypeOf(digestsAsString), digestsAsString)
			fmt.Printf("  PCRValues are %v => %v\n", reflect.TypeOf(ashex), ashex)
			pcrvs[i] = ashex

			//results = append(results, *pcrreadresponse)
		}
		banks[bankNames[b]] = pcrvs
	}

	return c.JSON(http.StatusOK, banks)
}

// IGNORE THIS CODE; IT WORKS SO I AM NOT TOUCHING IT

func xNewPCRs(c echo.Context) error {
	fmt.Println("NEW tpm2 pcrs called")

	tpm, err := openTPM("/dev/tpmrm0")
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open tpm with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	fmt.Printf("TPM device open by linuxtranport is %v\n", tpm)

	s1 := tpm2.TPMSPCRSelection{Hash: tpm2.TPMAlgSHA1, PCRSelect: tpm2.PCClientCompatible.PCRs(0)}
	s2 := tpm2.TPMSPCRSelection{Hash: tpm2.TPMAlgSHA256, PCRSelect: tpm2.PCClientCompatible.PCRs(0)}

	pcrselections := []tpm2.TPMSPCRSelection{s1, s2}
	selection := tpm2.TPMLPCRSelection{PCRSelections: pcrselections}
	fmt.Printf("PCR selection is  %v\n", selection)

	pcrreadresponse, err := tpm2.PCRRead{PCRSelectionIn: selection}.Execute(tpm)
	fmt.Printf("PCR pcrreadresponse is %w, %v\n", err, pcrreadresponse)

	//tpm2.PCRSelections{[]tpm2.PCRSelection{Hash: "sha256", PCRSelect: []byte{0, 1, 2, 3}}}

	return c.JSON(http.StatusOK, pcrreadresponse)
}
