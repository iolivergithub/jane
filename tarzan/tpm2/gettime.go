//go:build !windows

package tpm2

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// GetTime runs TPM2_GetTime: the TPM's clock, reset and restart counters and
// firmware version, signed by the AK and qualified by Jane's nonce (#68).
// Unlike a quote it involves no PCRs, so it is cheap to run often; across
// claims it shows reboots, TPM resets and whether the TPM clock runs
// forwards.
//
// Parameters:
//
//	tpm2/device    TPM device, default /dev/tpm0
//	tpm2/akhandle  AK handle, e.g. 0x810100AA
//	tpm2/nonce     qualifying data, base64
//
// The endorsement hierarchy (the privacy administrator) is authorised with
// the empty password: go-tpm (v0.9.8) declares GetTime's PrivacyAdminHandle as
// a plain handle, so no other password can be given. With an endorsement
// password set (JANE_TPM_ENDORSEMENT_PASSWORD) gettime therefore fails with an
// authorisation error; activatecredential and jp do use the password (#76).
func GetTime(c echo.Context) error {

	fmt.Println("tpm2 gettime called")

	ps := new(map[string]interface{})
	if err := c.Bind(&ps); err != nil {
		fmt.Printf("GetTime BIND err %v\n", err.Error())
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not decode parameters %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	params := *ps

	// All the parameters are checked before the TPM is opened
	tpm2device := tpmDeviceParam(params)
	akh, err := handleParam(params, "tpm2/akhandle")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	nonce, err := base64Param(params, "tpm2/nonce")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}

	tpm, err := OpenTPM(tpm2device)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open specified TPM %v during GetTime function with error %v", tpm2device, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			fmt.Printf("\ncan't close TPM %q: %v", tpm2device, err)
		}
	}()

	rtn, err := getTime(tpm, akh, nonce)
	if err != nil {
		fmt.Printf("Could not get the time with error %v\n", err.Error())
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	return c.JSON(http.StatusOK, rtn)
}

// getTime does the TPM part of GetTime.
func getTime(tpm transport.TPM, akh tpm2.TPMHandle, nonce []byte) (tpm2gettimeReturn, error) {
	ak, err := tpm2.ReadPublic{ObjectHandle: akh}.Execute(tpm)
	if err != nil {
		return tpm2gettimeReturn{}, fmt.Errorf("Could not read the AK at %v with error %v", handleHex(akh), err.Error())
	}

	rsp, err := tpm2.GetTime{
		PrivacyAdminHandle: tpm2.TPMRHEndorsement,
		SignHandle:         tpm2.NamedHandle{Handle: akh, Name: ak.Name},
		QualifyingData:     tpm2.TPM2BData{Buffer: nonce},
		InScheme:           quoteScheme(),
	}.Execute(tpm)
	if err != nil {
		return tpm2gettimeReturn{}, fmt.Errorf("Could not get the time with AK %v with error %v", handleHex(akh), err.Error())
	}

	attest, err := rsp.TimeInfo.Contents()
	if err != nil {
		return tpm2gettimeReturn{}, fmt.Errorf("Could not decode the time attestation with error %v", err.Error())
	}
	info, err := attest.Attested.Time()
	if err != nil {
		return tpm2gettimeReturn{}, fmt.Errorf("The attestation is not a time attestation: %v", err.Error())
	}

	return tpm2gettimeReturn{
		Time: timeStructure{
			Magic:           fmt.Sprintf("%0x", attest.Magic),
			Type:            fmt.Sprintf("%0x", attest.Type),
			QualifiedSigner: base64.StdEncoding.EncodeToString(attest.QualifiedSigner.Buffer),
			ExtraData:       base64.StdEncoding.EncodeToString(attest.ExtraData.Buffer),
			FirmwareVersion: fmt.Sprintf("%x", attest.FirmwareVersion),
			ClockInfo:       makeClockInfo(attest.ClockInfo),
			Time:            fmt.Sprintf("%v", info.Time.Time),
		},
		Attest:       base64.StdEncoding.EncodeToString(rsp.TimeInfo.Bytes()),
		Signature:    rsp.Signature,
		RawSignature: base64.StdEncoding.EncodeToString(tpm2.Marshal(rsp.Signature)),
	}, nil
}

// quoteScheme is the signing scheme tarzan uses for quotes and GetTime:
// RSASSA with SHA-256.
func quoteScheme() tpm2.TPMTSigScheme {
	return tpm2.TPMTSigScheme{
		Scheme: tpm2.TPMAlgRSASSA,
		Details: tpm2.NewTPMUSigScheme(
			tpm2.TPMAlgRSASSA,
			&tpm2.TPMSSchemeHash{HashAlg: tpm2.TPMAlgSHA256},
		),
	}
}

func makeClockInfo(ci tpm2.TPMSClockInfo) clockInfo {
	return clockInfo{
		fmt.Sprintf("%v", ci.Clock),
		fmt.Sprintf("%v", ci.ResetCount),
		fmt.Sprintf("%v", ci.RestartCount),
		fmt.Sprintf("%v", ci.Safe),
	}
}
