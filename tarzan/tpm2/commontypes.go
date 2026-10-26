package tpm2

import (
	"fmt"

	"github.com/google/go-tpm/tpm2"
)

type tpm2taErrorReturn struct {
	TPM2taError string `json:"tpm2taerror"`
}

type clockInfo struct {
	Clock        string `json:"clock"`
	ResetCount   string `json:"resetcount"`
	RestartCount string `json:"restartcount"`
	Safe         string `json:"safe"`
}

type attested struct {
	PCRSelect string `json:"pcrselect"`
	PCRDigest string `json:"pcrdigest"`
}

type quoteStructure struct {
	Magic           string    `json:"magic"`
	Type            string    `json:"type"`
	QualifiedSigner string    `json:"qualifiedsigner"`
	ExtraData       string    `json:"extradata"`
	ClockInfo       clockInfo `json:"clockinfo"`
	FirmwareVersion string    `json:"firmwareVersion"`
	Attested        attested  `json:"attested"`
}

// tpm2quoteReturn is the result of tpm2/quote. Attest (the raw TPMS_ATTEST)
// and RawSignature (the raw TPMT_SIGNATURE), both base64, are what the
// signature is verified over (#62).
type tpm2quoteReturn struct {
	Quote        quoteStructure `json:"quote"`
	Signature    interface{}    `json:"signature"`
	Attest       string         `json:"attest"`
	RawSignature string         `json:"rawsignature"`
}

// timeStructure is the decoded TPMS_ATTEST of TPM2_GetTime (#68); Time is
// the TPM's time in milliseconds since its time circuit was last reset.
type timeStructure struct {
	Magic           string    `json:"magic"`
	Type            string    `json:"type"`
	QualifiedSigner string    `json:"qualifiedsigner"`
	ExtraData       string    `json:"extradata"`
	ClockInfo       clockInfo `json:"clockinfo"`
	FirmwareVersion string    `json:"firmwareVersion"`
	Time            string    `json:"time"`
}

// tpm2gettimeReturn is the result of tpm2/gettime, shaped like a quote's.
type tpm2gettimeReturn struct {
	Time         timeStructure `json:"time"`
	Signature    interface{}   `json:"signature"`
	Attest       string        `json:"attest"`
	RawSignature string        `json:"rawsignature"`
}

// tpm2activateCredentialReturn is the result of tpm2/activatecredential: the
// recovered secret (base64) and the names of the keys used (hex).
type tpm2activateCredentialReturn struct {
	CertInfo string `json:"certinfo"`
	AKName   string `json:"akname"`
	EKName   string `json:"ekname"`
}

var npcrbanks = []tpm2.TPMIAlgHash{tpm2.TPMAlgSHA1, tpm2.TPMAlgSHA256, tpm2.TPMAlgSHA384, tpm2.TPMAlgSHA512}

type pcrValue map[int]string

var bankNames = map[tpm2.TPMAlgID]string{
	tpm2.TPMAlgSHA1:   "sha1",
	tpm2.TPMAlgSHA256: "sha256",
	tpm2.TPMAlgSHA384: "sha384",
	tpm2.TPMAlgSHA512: "sha512",
}

var bankValues = map[string]tpm2.TPMAlgID{
	"sha1":   tpm2.TPMAlgSHA1,
	"sha256": tpm2.TPMAlgSHA256,
	"sha384": tpm2.TPMAlgSHA384,
	"sha512": tpm2.TPMAlgSHA512,
}

// defaultTPMDevice is used when a request does not name a TPM device.
const defaultTPMDevice = "/dev/tpm0"

// tpmDeviceParam returns the "tpm2/device" parameter, or defaultTPMDevice when
// it is missing, empty or not a string, rather than panicking on the type
// assertion.
func tpmDeviceParam(params map[string]interface{}) string {
	if d, ok := params["tpm2/device"].(string); ok && d != "" {
		return d
	}
	return defaultTPMDevice
}

// firstDigestHex returns the first digest of a PCRRead response as hex. A bank
// can be allocated but not populated, e.g. SHA1 on modern TPMs and some vTPMs
// (QEMU, Proxmox): the read then succeeds with no digests, so ok is false
// instead of indexing an empty slice (#15).
func firstDigestHex(digests []tpm2.TPM2BDigest) (hexValue string, ok bool) {
	if len(digests) == 0 {
		return "", false
	}
	return fmt.Sprintf("%x", digests[0].Buffer), true
}
