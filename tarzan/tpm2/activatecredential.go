//go:build !windows

package tpm2

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// ActivateCredential runs TPM2_ActivateCredential on a credential made by Jane
// (TPM2_MakeCredential, done in software by the attestation server) and returns
// the recovered secret. The TPM only releases it if the EK at tpm2/ekhandle
// can decrypt the credential and the object at tpm2/akhandle has the name the
// credential was made for, i.e. the AK and EK are in the same TPM (#49).
//
// Parameters:
//
//	tpm2/device           TPM device, default /dev/tpm0
//	tpm2/akhandle         AK handle, e.g. 0x810100AA
//	tpm2/ekhandle         EK handle, e.g. 0x810100EE
//	tpm2/credentialblob   TPM2B_ID_OBJECT contents, base64
//	tpm2/encryptedsecret  TPM2B_ENCRYPTED_SECRET contents, base64
//
// Any other parameters are ignored.
func ActivateCredential(c echo.Context) error {

	fmt.Println("tpm2 activatecredential called")

	// Obtain the parameters
	ps := new(map[string]interface{})

	if err := c.Bind(&ps); err != nil {
		fmt.Printf("ActivateCredential BIND err %v\n", err.Error())
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
	ekh, err := handleParam(params, "tpm2/ekhandle")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	blob, err := base64Param(params, "tpm2/credentialblob")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	secret, err := base64Param(params, "tpm2/encryptedsecret")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}

	tpm, err := OpenTPM(tpm2device)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open specified TPM %v during ActivateCredential function with error %v", tpm2device, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			fmt.Printf("\ncan't close TPM %q: %v", tpm2device, err)
		}
	}()

	rtn, err := activateCredential(tpm, akh, ekh, blob, secret)
	if err != nil {
		fmt.Printf("Could not activate credential with error %v\n", err.Error())
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}

	return c.JSON(http.StatusOK, rtn)
}

// activateCredential does the TPM part of ActivateCredential. The names of
// both keys are read from the TPM: the EK's is needed for its policy session,
// and both are returned to help when an activation fails.
func activateCredential(tpm transport.TPM, akh, ekh tpm2.TPMHandle, blob, secret []byte) (tpm2activateCredentialReturn, error) {
	ak, err := tpm2.ReadPublic{ObjectHandle: akh}.Execute(tpm)
	if err != nil {
		return tpm2activateCredentialReturn{}, fmt.Errorf("Could not read the AK at %v with error %v", handleHex(akh), err.Error())
	}
	ek, err := tpm2.ReadPublic{ObjectHandle: ekh}.Execute(tpm)
	if err != nil {
		return tpm2activateCredentialReturn{}, fmt.Errorf("Could not read the EK at %v with error %v", handleHex(ekh), err.Error())
	}

	// The AK is authorised with its (empty) password, the EK with its policy
	rsp, err := tpm2.ActivateCredential{
		ActivateHandle: tpm2.AuthHandle{Handle: akh, Name: ak.Name, Auth: tpm2.PasswordAuth(nil)},
		KeyHandle:      tpm2.AuthHandle{Handle: ekh, Name: ek.Name, Auth: ekSession()},
		CredentialBlob: tpm2.TPM2BIDObject{Buffer: blob},
		Secret:         tpm2.TPM2BEncryptedSecret{Buffer: secret},
	}.Execute(tpm)
	if err != nil {
		return tpm2activateCredentialReturn{}, fmt.Errorf("Could not activate credential for AK %v with EK %v with error %v", handleHex(akh), handleHex(ekh), err.Error())
	}

	return tpm2activateCredentialReturn{
		CertInfo: base64.StdEncoding.EncodeToString(rsp.CertInfo.Buffer),
		AKName:   hex.EncodeToString(ak.Name.Buffer),
		EKName:   hex.EncodeToString(ek.Name.Buffer),
	}, nil
}

// ekSession satisfies the authorisation policy of an EK made from the
// standard (low range) template, TPM2_PolicySecret(TPM_RH_ENDORSEMENT) with
// the endorsement password (empty unless JANE_TPM_ENDORSEMENT_PASSWORD is set). A policy session is single use, so each
// command needs a fresh one. Same as the provisioner's.
func ekSession() tpm2.Session {
	return tpm2.Policy(tpm2.TPMAlgSHA256, 16,
		func(t transport.TPM, s tpm2.TPMISHPolicy, nonce tpm2.TPM2BNonce) error {
			_, err := tpm2.PolicySecret{
				AuthHandle:    tpm2.AuthHandle{Handle: tpm2.TPMRHEndorsement, Auth: endorsementAuth()},
				PolicySession: s,
				NonceTPM:      nonce,
			}.Execute(t)
			return err
		})
}

// handleParam returns the TPM handle in params[key], written in hex with or
// without 0x, e.g. 0x810100AA.
func handleParam(params map[string]interface{}, key string) (tpm2.TPMHandle, error) {
	s, ok := params[key].(string)
	if !ok || s == "" {
		return 0, fmt.Errorf("Missing parameter %v", key)
	}
	h, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 32)
	if err != nil {
		return 0, fmt.Errorf("Unable to parse %v %v as a hex handle", key, s)
	}
	return tpm2.TPMHandle(h), nil
}

// base64Param returns the standard base64 parameter params[key], decoded.
func base64Param(params map[string]interface{}, key string) ([]byte, error) {
	s, ok := params[key].(string)
	if !ok || s == "" {
		return nil, fmt.Errorf("Missing parameter %v", key)
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("Unable to base64 decode %v with error %v", key, err.Error())
	}
	return b, nil
}

func handleHex(h tpm2.TPMHandle) string {
	return fmt.Sprintf("0x%08X", uint32(h))
}
