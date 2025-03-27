//go:build !windows

package tpm2

import (
	"bytes"
	"encoding/base64"
	_ "encoding/hex"
	"fmt"
	_ "log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

func NewQuote(c echo.Context) error {

	// Honestly this code is freaking awful
	// but probably not as bad as PCRReadResponse

	fmt.Println("NEW quote called")

	// Obtain the parameters
	ps := new(map[string]interface{})

	if err := c.Bind(&ps); err != nil {
		fmt.Printf("NewQuote BIND err %w\n", err)
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
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open TPM during Quote function with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}

	// Here we parse the pcrSelection to obtain the []int structure for the pcrselections
	s := strings.Split(params["pcrSelection"].(string), ",")
	fmt.Println("pcr selection string: %v\n", s)
	pcrsel := make([]int, len(s), len(s))
	for i, r := range s {
		v64, err := strconv.ParseUint(r, 10, 8)

		if err != nil {
			pcrsel[i] = 0
		} else {
			pcrsel[i] = int(v64)
		}
	}

	// Here we parse the bank
	b := params["bank"].(string)
	pcrbank := bankValues[b]
	fmt.Printf("pcrbank %v\n", pcrbank)

	// Here we parse the nonce
	// If none then one will be generated
	nonce := params["tpm2/nonce"].(string)

	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not base64 decode nonce: %v", err.Error())}
		return c.JSON(http.StatusInternalServerError, rtn)
	}
	nonceTPM2B := tpm2.TPM2BData{Buffer: nonceBytes}

	// Here we parse the akhandle
	// This is a bit ugly but...that's the way go does things
	// Strip the 0x, parse it as a Uint in base 16 with size 32 - returns a unit64, convert to a uint32 and then create the TPM handle
	akh := strings.Replace(params["tpm2/akhandle"].(string), "0x", "", -1)
	fmt.Printf("akh is %v\n", akh)

	h, err := strconv.ParseUint(akh, 16, 32)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Unable to parse AK handle %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	h32 := uint32(h) // this is safe because we only create a 32bit unsigned value above.

	//var signingHandle tpmutil.Handle = tpmutil.Handle(h32)
	fmt.Printf("handle is %v\n", h32)
	signingHandle := tpmutil.Handle(h32)
	namedHandle := tpm2.NamedHandle{
		Handle: tpm2.TPMHandle(signingHandle),
		Name:   tpm2.TPM2BName{}, // You may need to set this to an appropriate value
	}

	scheme := tpm2.TPMTSigScheme{
		Scheme: tpm2.TPMAlgRSASSA,
		Details: tpm2.NewTPMUSigScheme(
			tpm2.TPMAlgRSASSA,
			&tpm2.TPMSSchemeHash{
				HashAlg: tpm2.TPMAlgSHA256,
			},
		),
	}

	// PCR selection (selecting PCR 7 for this example)
	pcrSelection := tpm2.TPMLPCRSelection{
		PCRSelections: []tpm2.TPMSPCRSelection{
			{
				Hash:      pcrbank,
				PCRSelect: tpm2.PCClientCompatible.PCRs(7),
			},
		},
	}

	// Here's the quote

	quoteresponse, err := tpm2.Quote{SignHandle: namedHandle, QualifyingData: nonceTPM2B, InScheme: scheme, PCRSelect: pcrSelection}.Execute(tpm)
	if err != nil {
		fmt.Printf("Could not make Quote with error %v\n", err.Error())

		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not make Quote with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}

	q := *quoteresponse
	quotepart := q.Quoted
	signaturepart := q.Signature

	fmt.Printf("Quoted is %v\nand Signature is %v\n", quotepart, signaturepart)
	fmt.Printf("Types are %v and %v\n", reflect.TypeOf(quotepart), reflect.TypeOf(signaturepart))
	fmt.Printf("attempting the decode")
	_, err = DecodeTPM2BAttest(quotepart)
	fmt.Printf("and we got back")

	//fmt.Printf("decoded attest %v, %v\n", err, dtpm2attest)

	qr := tpm2quoteReturn{quotepart, signaturepart}

	return c.JSON(http.StatusOK, qr)
}

func DecodeTPM2BAttest(attestData tpm2.TPM2BAttest) (*tpm2.TPMSAttest, error) {
	// Access the buffer using the Bytes() method
	dataBuf := bytes.NewBuffer(attestData.Bytes())
	//fmt.Printf("databuf=%v\n", dataBuf)
	fmt.Printf("we got the databuf\n")
	var attestationData tpm2.TPMSAttest
	if err := tpmutil.UnpackBuf(dataBuf, &attestationData); err != nil {
		fmt.Printf("error is %w\n", err)
		return nil, fmt.Errorf("unmarshalling attestation data: %w", err)
	}
	return &attestationData, nil
}
