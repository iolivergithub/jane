package tpm2rules

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"a10/structures"
	"a10/utilities"

	"github.com/google/go-tpm/tpm2"
)

// Rules and helpers for the signed part of tpm2/quote and tpm2/gettime claims
// (#83). Since 1.5.3 tarzan returns, for both, the raw TPMS_ATTEST the TPM
// signed ("attest") and its TPMT_SIGNATURE ("rawsignature"), base64. The
// decoded fields ("quote", "time") are a convenience copy that the signature
// does not cover.

// attestType is the TPMS_ATTEST type each signed function must carry.
var attestType = map[string]tpm2.TPMISTAttest{
	"tpm2/quote":   tpm2.TPMSTAttestQuote, // 0x8018
	"tpm2/gettime": tpm2.TPMSTAttestTime,  // 0x8019
}

// signedFunction checks the claim is a tpm2/quote or tpm2/gettime claim.
func signedFunction(claim structures.Claim) (string, string) {
	f := claim.Header.Intent.Function
	if _, ok := attestType[f]; !ok {
		return f, fmt.Sprintf("Rule applies to tpm2/quote and tpm2/gettime claims, not %s", f)
	}
	return f, ""
}

// signedAttest decodes the claim's raw attest and signature (no verification).
// On failure it returns the reason as a non-empty message.
func signedAttest(claim structures.Claim) (*utilities.AttestableData, string) {
	a, _ := claim.Body["attest"].(string)
	s, _ := claim.Body["rawsignature"].(string)
	if a == "" || s == "" {
		return nil, "Claim has no attest/rawsignature (made by a tarzan older than 1.5.3?)"
	}
	ab, err := base64.StdEncoding.DecodeString(a)
	if err != nil {
		return nil, "attest is not valid base64"
	}
	sb, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, "rawsignature is not valid base64"
	}
	var d utilities.AttestableData
	if err := d.Decode(ab, sb); err != nil {
		return nil, fmt.Sprintf("attest/rawsignature could not be decoded: %v", err)
	}
	return &d, ""
}

// ValidSignature checks the signature over the claim's raw attest with the
// AK, and that the attest is of the type the claim's function produces. The
// AK is the element's (tpm2.ak.public) unless the caller passed tpm2/akpublic.
func ValidSignature(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	f, msg := signedFunction(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	d, msg := signedAttest(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	if d.Data.Type != attestType[f] {
		return structures.Fail, fmt.Sprintf("Signed attest is of type %x, but a %s claim must be %x", uint16(d.Data.Type), f, uint16(attestType[f])), nil
	}

	akb64, source := claim.Header.Element.TPM2.AK.Public, "element tpm2.ak.public"
	if p, ok := claim.Header.CallParameters["tpm2/akpublic"].(string); ok && p != "" {
		akb64, source = p, "tpm2/akpublic call parameter"
	}
	if akb64 == "" {
		return structures.Fail, "No AK public key: the element has no tpm2.ak.public (run jp update)", nil
	}
	akb, err := base64.StdEncoding.DecodeString(akb64)
	if err != nil {
		return structures.Fail, "AK public key (" + source + ") is not valid base64", nil
	}
	ak, err := utilities.ParseTPMKey(akb)
	if err != nil {
		return structures.Fail, fmt.Sprintf("AK public key (%s) could not be used: %v (only RSA AKs are supported)", source, err), nil
	}

	if err := d.VerifySignature(ak); err != nil {
		return structures.Fail, fmt.Sprintf("Signature does not verify with the AK from the %s: %v", source, err), nil
	}
	return structures.Success, fmt.Sprintf("Signature (%s) verifies with the AK at %v, from the %s",
		sigAlgName(d.Signature.SigAlg), akHandle(claim), source), nil
}

func akHandle(claim structures.Claim) interface{} {
	if h, ok := claim.Header.CallParameters["tpm2/akhandle"]; ok {
		return h
	}
	return claim.Header.Element.TPM2.AK.Handle
}

func sigAlgName(a tpm2.TPMIAlgSigScheme) string {
	switch a {
	case tpm2.TPMAlgRSASSA:
		return "RSASSA"
	case tpm2.TPMAlgECDSA:
		return "ECDSA"
	}
	return fmt.Sprintf("algorithm 0x%x", uint16(a))
}

// clockValues are the TPM clock fields of a quote or gettime claim.
type clockValues struct {
	Clock        uint64
	ResetCount   uint32
	RestartCount uint32
	Safe         bool
	Firmware     uint64
	FromAttest   bool // false: taken from the decoded copy (no attest in the claim)
}

// claimClock reads the clock fields from the signed attest, or, for claims
// without one (tarzan < 1.5.3), from the decoded quote/time body.
func claimClock(claim structures.Claim) (clockValues, string) {
	if a, _ := claim.Body["attest"].(string); a != "" {
		ab, err := base64.StdEncoding.DecodeString(a)
		if err != nil {
			return clockValues{}, "attest is not valid base64"
		}
		att, err := tpm2.Unmarshal[tpm2.TPMSAttest](ab)
		if err != nil {
			return clockValues{}, fmt.Sprintf("attest could not be decoded: %v", err)
		}
		ci := att.ClockInfo
		return clockValues{ci.Clock, ci.ResetCount, ci.RestartCount, bool(ci.Safe), att.FirmwareVersion, true}, ""
	}

	key := "quote"
	if claim.Header.Intent.Function == "tpm2/gettime" {
		key = "time"
	}
	body, _ := claim.Body[key].(map[string]interface{})
	ci, _ := body["clockinfo"].(map[string]interface{})
	if body == nil || ci == nil {
		return clockValues{}, fmt.Sprintf("Claim has neither attest nor %s.clockinfo", key)
	}
	var v clockValues
	var err error
	str := func(m map[string]interface{}, k string) string { s, _ := m[k].(string); return s }
	if v.Clock, err = strconv.ParseUint(str(ci, "clock"), 10, 64); err != nil {
		return v, "clockinfo.clock is not a number"
	}
	r, err := strconv.ParseUint(str(ci, "resetcount"), 10, 32)
	if err != nil {
		return v, "clockinfo.resetcount is not a number"
	}
	s, err := strconv.ParseUint(str(ci, "restartcount"), 10, 32)
	if err != nil {
		return v, "clockinfo.restartcount is not a number"
	}
	v.ResetCount, v.RestartCount = uint32(r), uint32(s)
	v.Safe = str(ci, "safe") == "true"
	if v.Firmware, err = strconv.ParseUint(str(body, "firmwareVersion"), 16, 64); err != nil {
		return v, "firmwareVersion is not a hex number"
	}
	return v, ""
}
