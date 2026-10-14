package tpm2rules

import (
	"encoding/base64"
	"fmt"
	"strings"

	"a10/structures"
	"a10/utilities"

	"github.com/google/go-tpm/tpm2"
)

// Rules for tpm2/activatecredential claims (#49). Neither needs an expected
// value: what they check against is in the claim's call parameters, which
// hold the element's AK and EK unless they were overridden.

// CredentialActivated checks the secret the TPM recovered has the SHA-256 Jane
// kept when it made the credential, i.e. the AK is in the same TPM as the EK.
func CredentialActivated(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	want, ok := claim.Header.CallParameters["tpm2/credentialsecrethash"].(string)
	if !ok || want == "" {
		return structures.Fail, "Claim has no tpm2/credentialsecrethash call parameter - not a tpm2/activatecredential claim?", nil
	}

	certinfo, ok := (claim.Body)["certinfo"].(string)
	if !ok {
		return structures.Fail, "Claim does not contain certinfo", nil
	}
	secret, err := base64.StdEncoding.DecodeString(certinfo)
	if err != nil {
		return structures.Fail, "Decoding certinfo failed", err
	}

	if got := utilities.CredentialSecretHash(secret); got != want {
		return structures.Fail, fmt.Sprintf("Recovered secret does not match: SHA-256 %v, expected %v", got, want), nil
	}

	msg := fmt.Sprintf("Credential activated: AK %v and EK %v are in the same TPM",
		claim.Header.CallParameters["tpm2/akhandle"], claim.Header.CallParameters["tpm2/ekhandle"])
	return structures.Success, msg, nil
}

// AKAttributes checks the AK the credential was made for is a restricted
// signing key that never leaves its TPM. Activation does not check this, and
// without it the key could sign data made to look like a quote.
func AKAttributes(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	b64, _ := claim.Header.CallParameters["tpm2/akpublic"].(string)
	ak, err := utilities.ParseTPMPublic(b64)
	if err != nil {
		return structures.Fail, "Parsing tpm2/akpublic from the call parameters failed", err
	}

	if problems := akAttributeProblems(ak.ObjectAttributes); len(problems) > 0 {
		return structures.Fail, "AK is not a proper attestation key: " + strings.Join(problems, ", "), nil
	}
	return structures.Success, "AK is a restricted signing key with fixedTPM, fixedParent and sensitiveDataOrigin", nil
}

// akAttributeProblems lists what is wrong with an AK's attributes.
func akAttributeProblems(a tpm2.TPMAObject) []string {
	var problems []string
	if !a.Restricted {
		problems = append(problems, "not restricted")
	}
	if !a.SignEncrypt {
		problems = append(problems, "not a signing key")
	}
	if a.Decrypt {
		problems = append(problems, "can decrypt")
	}
	if !a.FixedTPM {
		problems = append(problems, "not fixedTPM")
	}
	if !a.FixedParent {
		problems = append(problems, "not fixedParent")
	}
	if !a.SensitiveDataOrigin {
		problems = append(problems, "not sensitiveDataOrigin")
	}
	return problems
}
