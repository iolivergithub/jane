//go:build !windows

package tpm2

import (
	"encoding/base64"
	"fmt"

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
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open specified TPM %v during Quote function with error %v", tpm2device, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}

	// Here we parse the bank
	b := params["bank"].(string)
	pcrbank := bankValues[b]
	fmt.Printf("pcrbank %v\n", pcrbank)

	// These need to be integrated to generate the PCRSelections, of type []tpm2.TPMSPCRSelection

	// Here we parse the pcrSelection to obtain the []int structure for the pcrselections
	s := strings.Split(params["pcrSelection"].(string), ",")
	fmt.Printf("pcr selection string: %v\n", s)

	// *** This is the old code which does not work
	// pcrselectionlist := []tpm2.TPMSPCRSelection{}
	// for _, r := range s {
	// 	v64, err := strconv.ParseUint(r, 10, 8)
	// 	fmt.Printf("creating pcrselection for %v,%v,%v\n", err, pcrbank, v64)
	// 	if err == nil {
	// 		pcrsel := tpm2.TPMSPCRSelection{Hash: pcrbank, PCRSelect: tpm2.PCClientCompatible.PCRs(uint(v64))}
	// 		pcrselectionlist = append(pcrselectionlist, pcrsel)
	// 	}
	// }
	// ***

	var indcies = make([]uint, len(s))
	for i, r := range s {
		v, _ := strconv.ParseUint(r, 10, 8)
		indcies[i] = uint(v)
		// TODO err
	}
	pcrSelection := tpm2.TPMSPCRSelection{
		Hash:      pcrbank,
		PCRSelect: tpm2.PCClientCompatible.PCRs(indcies...),
	}

	pcrselectionlist := tpm2.TPMLPCRSelection{PCRSelections: []tpm2.TPMSPCRSelection{pcrSelection}}

	// *** This is the old code which does not work
	// PCR selection (selecting PCR 7 for this example)
	// pcrSelection := tpm2.TPMLPCRSelection{
	//	PCRSelections: pcrselectionlist,
	// }
	//***

	fmt.Printf("PCRselectionlist is %v\n", pcrselectionlist)

	// Here we parse the nonce
	// If none then one will be generated
	nonce := params["tpm2/nonce"].(string)
	fmt.Printf("Received nonce is %v\n", nonce)
	decodedNonce, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Unable base64 undecode provided nonce %v with error %v", nonce, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	nonceTPM2B := tpm2.TPM2BData{Buffer: decodedNonce}

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

	signingHandle := tpmutil.Handle(h32)
	namedHandle := tpm2.NamedHandle{
		Handle: tpm2.TPMHandle(signingHandle),
		Name:   tpm2.TPM2BName{}, // This seems to work....?
	}

	// This sets up the signing scheme, which is always RSASSA/SHA256

	scheme := tpm2.TPMTSigScheme{
		Scheme: tpm2.TPMAlgRSASSA,
		Details: tpm2.NewTPMUSigScheme(
			tpm2.TPMAlgRSASSA,
			&tpm2.TPMSSchemeHash{
				HashAlg: tpm2.TPMAlgSHA256,
			},
		),
	}

	// Here's the quote

	quoteresponse, err := tpm2.Quote{SignHandle: namedHandle, QualifyingData: nonceTPM2B, InScheme: scheme, PCRSelect: pcrselectionlist}.Execute(tpm)
	if err != nil {
		fmt.Printf("Could not make Quote with error %v\n", err.Error())

		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not make Quote with error %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}

	q := *quoteresponse
	var quotepart tpm2.TPM2B[tpm2.TPMSAttest, *tpm2.TPMSAttest]
	quotepart = q.Quoted         // see type above :-)
	signaturepart := q.Signature // of type tpm2.TPMSSignature
	quotecontents, _ := quotepart.Contents()

	fmt.Println("********************")
	fmt.Printf("Received nonce is %v\n", nonce)
	fmt.Printf(" type       : %v\n", reflect.TypeOf(quotecontents.ExtraData))
	fmt.Printf("buffer type : %v\n", reflect.TypeOf(quotecontents.ExtraData.Buffer))
	fmt.Printf("as hex : %x\n", quotecontents.ExtraData.Buffer)
	fmt.Printf("as str : %v\n", string(quotecontents.ExtraData.Buffer))
	fmt.Printf("as nor : %v\n\n", quotecontents.ExtraData.Buffer)
	fmt.Println("********************")

	quoteinfo, _ := quotecontents.Attested.Quote()
	attested := attested{
		fmt.Sprintf("%v", quoteinfo.PCRSelect),
		//fmt.Sprintf("%x", quoteinfo.PCRDigest.Buffer),
		fmt.Sprintf("%v", base64.StdEncoding.EncodeToString(quoteinfo.PCRDigest.Buffer)),
	}
	clockinfo := clockInfo{
		fmt.Sprintf("%v", quotecontents.ClockInfo.Clock),
		fmt.Sprintf("%v", quotecontents.ClockInfo.ResetCount),
		fmt.Sprintf("%v", quotecontents.ClockInfo.RestartCount),
		fmt.Sprintf("%v", quotecontents.ClockInfo.Safe),
	}

	fmt.Printf("Quote contents\n Magic, Type %v,%v \n", quotecontents.Magic, quotecontents.Type)

	qstr := quoteStructure{
		//Magic: string(quotecontents.Magic),
		Magic:           fmt.Sprintf("%0x", quotecontents.Magic),
		Type:            fmt.Sprintf("%0x", quotecontents.Type),
		QualifiedSigner: fmt.Sprintf("%x", base64.StdEncoding.EncodeToString(quotecontents.QualifiedSigner.Buffer)),
		ExtraData:       fmt.Sprintf("%v", base64.StdEncoding.EncodeToString(quotecontents.ExtraData.Buffer)),
		FirmwareVersion: fmt.Sprintf("%x", quotecontents.FirmwareVersion),
		ClockInfo:       clockinfo,
		Attested:        attested,
	}
	fmt.Printf("QSTR is %v\n", qstr)

	return c.JSON(http.StatusOK, tpm2quoteReturn{qstr, signaturepart})
}
