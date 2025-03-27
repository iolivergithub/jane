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
)

func NewPCRs(c echo.Context) error {

	// Honestly this code is freaking awful
	// gotpm PCRRead only accepts 8 PCR values at a time, but we want them all which requires
	// multiple calls...so we go through each possible bank, one at a time and take one PCR
	// at a time and dump them all into a struct (banks)...ugly, it works, but awful
	// Oh, and if the PCRs get modified during the call of this function, then the results
	// might be inconsistent, but then again PCRs in this form aren't signed so good luck

	//results := []tpm2.PCRReadResponse{}
	banks := make(map[string]pcrValue)

	fmt.Println("NEW tpm2 pcrs called")

	// Obtain the parameters
	ps := new(map[string]interface{})

	if err := c.Bind(&ps); err != nil {
		fmt.Printf("NewPCRs BIND err %w\n", err)
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not decode parameters %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}

	params := *ps
	fmt.Printf("%v\n", params)

	// Here we parse the tpm2 device
	// We have a default of /dev/tpm0
	tpm2device := params["tpm2/device"].(string)

	tpm, err := OpenTPM(tpm2device)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open TPM during PCRRead function with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			fmt.Printf("\ncan't close TPM %q: %v", tpm2device, err)
		}
	}()

	for _, b := range npcrbanks {
		pcrvs := make(map[int]string)
		for i := 0; i <= 23; i++ {
			//fmt.Printf("Reading back %v, pcr %v -->\n", b, i)

			s2 := tpm2.TPMSPCRSelection{Hash: b, PCRSelect: tpm2.PCClientCompatible.PCRs(uint(i))}

			pcrselections := []tpm2.TPMSPCRSelection{s2}
			selection := tpm2.TPMLPCRSelection{PCRSelections: pcrselections}
			//fmt.Printf("PCR selection is %v\n", selection)

			pcrreadresponse, err := tpm2.PCRRead{PCRSelectionIn: selection}.Execute(tpm)
			if err != nil {
				rtn := tpm2taErrorReturn{fmt.Sprintf("Could not read PCRs with error %v", err.Error())}
				return c.JSON(http.StatusUnprocessableEntity, rtn)
			}

			pcrvalues := *pcrreadresponse
			digests := pcrvalues.PCRValues.Digests[0]
			digestsAsString := digests.Buffer

			//fmt.Printf("PCR pcrreadresponse is %w, %v\n", err, pcrreadresponse)
			ashex := fmt.Sprintf("%x", digestsAsString)
			//fmt.Printf("  PCRValues are %v => %v\n", reflect.TypeOf(digestsAsString), digestsAsString)
			fmt.Printf("  PCRValues are %v => %v\n", reflect.TypeOf(ashex), ashex)
			pcrvs[i] = ashex

			//results = append(results, *pcrreadresponse)
		}
		banks[bankNames[b]] = pcrvs
	}

	return c.JSON(http.StatusOK, banks)
}

// IGNORE THIS CODE; IT WORKS SO I AM NOT TOUCHING IT

// func xNewPCRs(c echo.Context) error {
// 	fmt.Println("NEW tpm2 pcrs called")

// 	tpm, err := openTPM("/dev/tpmrm0")
// 	if err != nil {
// 		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open tpm with error %v", err.Error())}
// 		return c.JSON(http.StatusUnprocessableEntity, rtn)
// 	}
// 	fmt.Printf("TPM device open by linuxtranport is %v\n", tpm)

// 	s1 := tpm2.TPMSPCRSelection{Hash: tpm2.TPMAlgSHA1, PCRSelect: tpm2.PCClientCompatible.PCRs(0)}
// 	s2 := tpm2.TPMSPCRSelection{Hash: tpm2.TPMAlgSHA256, PCRSelect: tpm2.PCClientCompatible.PCRs(0)}

// 	pcrselections := []tpm2.TPMSPCRSelection{s1, s2}
// 	selection := tpm2.TPMLPCRSelection{PCRSelections: pcrselections}
// 	fmt.Printf("PCR selection is  %v\n", selection)

// 	pcrreadresponse, err := tpm2.PCRRead{PCRSelectionIn: selection}.Execute(tpm)
// 	fmt.Printf("PCR pcrreadresponse is %w, %v\n", err, pcrreadresponse)

// 	//tpm2.PCRSelections{[]tpm2.PCRSelection{Hash: "sha256", PCRSelect: []byte{0, 1, 2, 3}}}

// 	return c.JSON(http.StatusOK, pcrreadresponse)
// }
