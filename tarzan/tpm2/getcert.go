package tpm2

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// defaultEKCertHandle is the TCG NV index of the RSA 2048 EK certificate
// (TCG EK Credential Profile; the ECC P-256 one is at 0x01C0000A).
const defaultEKCertHandle = 0x01C00002

// tpm2getcertReturn is the result of tpm2/getcert (#63): the EK certificate
// as read from NV, DER in base64, and a summary of it.
type tpm2getcertReturn struct {
	Handle      string        `json:"handle"`
	Size        int           `json:"size"`
	Certificate string        `json:"certificate"`
	Summary     ekCertSummary `json:"summary"`
}

// GetCert reads the EK certificate from the TPM's NV storage. Nothing is
// signed: the certificate is checked by Jane's rules (tpm2_ekCertMatchesEK,
// tpm2_ekCertIssuer).
//
// Parameters:
//
//	tpm2/device         TPM device, default /dev/tpm0
//	tpm2/ekcerthandle   NV index of the certificate, default 0x01C00002
func GetCert(c echo.Context) error {

	fmt.Println("tpm2 getcert called")

	ps := new(map[string]interface{})
	if err := c.Bind(&ps); err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not decode parameters %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	params := *ps

	tpm2device := tpmDeviceParam(params)
	idx := tpm2.TPMHandle(defaultEKCertHandle)
	if s, _ := params["tpm2/ekcerthandle"].(string); s != "" {
		h, err := handleParam(params, "tpm2/ekcerthandle")
		if err != nil {
			return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
		}
		idx = h
	}
	if uint32(idx)>>24 != 0x01 {
		msg := fmt.Sprintf("tpm2/ekcerthandle %v is not an NV index (0x01xxxxxx); the RSA EK certificate is normally at 0x01C00002", handleHex(idx))
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{msg})
	}

	tpm, err := OpenTPM(tpm2device)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open specified TPM %v during GetCert function with error %v", tpm2device, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			fmt.Printf("\ncan't close TPM %q: %v", tpm2device, err)
		}
	}()

	data, err := readNV(tpm, idx)
	if err != nil {
		fmt.Printf("Could not read the EK certificate with error %v\n", err.Error())
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	der, err := trimDER(data)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{fmt.Sprintf("NV index %v: %v", handleHex(idx), err.Error())})
	}

	return c.JSON(http.StatusOK, tpm2getcertReturn{
		Handle:      handleHex(idx),
		Size:        len(der),
		Certificate: base64.StdEncoding.EncodeToString(der),
		Summary:     summariseEKCert(der),
	})
}

// readNV reads a whole NV index in chunks of at most TPM_PT_NV_BUFFER_MAX.
// EK certificate indices allow reading with their own (empty) authorisation
// (TPMA_NV_AUTHREAD); otherwise the owner hierarchy with the empty password
// is tried.
func readNV(tpm transport.TPM, idx tpm2.TPMHandle) ([]byte, error) {
	pubRsp, err := tpm2.NVReadPublic{NVIndex: idx}.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("No NV index at %v (no EK certificate there?): %v", handleHex(idx), err)
	}
	pub, err := pubRsp.NVPublic.Contents()
	if err != nil {
		return nil, fmt.Errorf("Could not decode the public area of NV index %v: %v", handleHex(idx), err)
	}

	auth := tpm2.AuthHandle{Handle: idx, Name: pubRsp.NVName, Auth: tpm2.PasswordAuth(nil)}
	if !pub.Attributes.AuthRead {
		auth = tpm2.AuthHandle{Handle: tpm2.TPMRHOwner, Auth: tpm2.PasswordAuth(nil)}
	}

	chunk := nvBufferMax(tpm)
	data := make([]byte, 0, pub.DataSize)
	for off := uint16(0); off < pub.DataSize; {
		n := pub.DataSize - off
		if n > chunk {
			n = chunk
		}
		rsp, err := tpm2.NVRead{
			AuthHandle: auth,
			NVIndex:    tpm2.NamedHandle{Handle: idx, Name: pubRsp.NVName},
			Size:       n,
			Offset:     off,
		}.Execute(tpm)
		if err != nil {
			return nil, fmt.Errorf("Could not read NV index %v at offset %d: %v", handleHex(idx), off, err)
		}
		data = append(data, rsp.Data.Buffer...)
		off += n
	}
	return data, nil
}

// nvBufferMax is the TPM's largest NV read, or 512 if it will not say.
func nvBufferMax(tpm transport.TPM) uint16 {
	rsp, err := tpm2.GetCapability{Capability: tpm2.TPMCapTPMProperties, Property: uint32(tpm2.TPMPTNVBufferMax), PropertyCount: 1}.Execute(tpm)
	if err == nil {
		if props, err := rsp.CapabilityData.Data.TPMProperties(); err == nil {
			for _, p := range props.TPMProperty {
				if p.Property == tpm2.TPMPTNVBufferMax && p.Value > 0 && p.Value <= 0xFFFF {
					return uint16(p.Value)
				}
			}
		}
	}
	return 512
}
