package tpm2rules

import (
	"fmt"

	"a10/structures"

	"github.com/mitchellh/mapstructure" // from Hasicorp https://stackoverflow.com/questions/26744873/converting-map-to-struct
)

func Registration() []structures.Rule {
	attestedPCRDigest := structures.Rule{"tpm2_attestedValue", "Checks the TPM's reported attested value against the expected value", AttestedPCRDigest, true}
	ruleFirmware := structures.Rule{"tpm2_firmware", "Checks the TPM firmware version against the expected value", FirmwareRule, true}
	ruleMagic := structures.Rule{"tpm2_magicNumber", "Checks the quote magic number is 0xFF544347", MagicNumberRule, false}
	ruleQuoteType := structures.Rule{"tpm2_type", "Checks the type of the quote which must be 0x8018", QuoteTypeRule, false}
	ruleIsSafe := structures.Rule{"tpm2_safe", "Checks that the value of safe is 1", IsSafe, false}
	ruleValidNonce := structures.Rule{"tpm2_validNonce", "Checks that nonce used for the claim matches the nonce in the quote", ValidNonce, false}
	ruleCredentialActivated := structures.Rule{"tpm2_credentialActivated", "Checks the TPM recovered the secret of a credential made for the AK and EK (tpm2/activatecredential)", CredentialActivated, false}
	ruleAKAttributes := structures.Rule{"tpm2_akAttributes", "Checks the AK used in tpm2/activatecredential is a restricted, fixedTPM signing key", AKAttributes, false}
	rulePCRDigest := structures.Rule{"tpm2_pcrDigest", "Checks the PCR values in the tpm2/pcrs claim given by parameter pcrscid give this quote's attested value", PCRDigest, false}
	ruleUEFILogReplay := structures.Rule{"tpm2_uefiLogReplay", "Replays the UEFI event log in the uefi/eventlog claim given by parameter uefilogcid against this tpm2/pcrs claim", UEFILogReplay, false}
	ruleValidSignature := structures.Rule{"tpm2_validSignature", "Checks the signature of a tpm2/quote or tpm2/gettime claim's signed attest with the AK, and its type", ValidSignature, false}
	ruleCapabilitiesFirmware := structures.Rule{"tpm2_capabilitiesFirmware", "Checks the firmware version in this tpm2/capabilities claim equals the signed one in the quote/gettime claim given by parameter quotecid", CapabilitiesFirmware, false}
	ruleClockProgress := structures.Rule{"tpm2_clockProgress", "Checks the TPM clock, reset and restart counts against the earlier quote/gettime claim given by parameter previouscid", ClockProgress, false}

	ruleEKCertMatchesEK := structures.Rule{"tpm2_ekCertMatchesEK", "Checks the EK certificate in this tpm2/getcert claim holds the element's EK (RSA)", EKCertMatchesEK, false}
	ruleEKCertIssuer := structures.Rule{"tpm2_ekCertIssuer", "Checks the EK certificate in this tpm2/getcert claim was issued by the certificates at the expected value's issuerurl", EKCertIssuer, true}

	return []structures.Rule{ruleFirmware, ruleMagic, attestedPCRDigest, ruleIsSafe, ruleValidNonce, ruleQuoteType, ruleCredentialActivated, ruleAKAttributes, rulePCRDigest, ruleUEFILogReplay, ruleValidSignature, ruleCapabilitiesFirmware, ruleClockProgress, ruleEKCertMatchesEK, ruleEKCertIssuer}
}

func IsSafe(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	msg := fmt.Sprintf("TPM safe value is %v", q.ClockInfo.Safe)

	if q.ClockInfo.Safe == "false" {
		return structures.Fail, "Uncommanded device/TPM shutdown. " + msg, nil
	}

	return structures.Success, "TPM shutdown normal. " + msg, nil
}

func AttestedPCRDigest(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	claimedAV := q.Attested.PCRDigest
	expectedAV := (ev.EVS)["attestedValue"]

	if expectedAV == claimedAV {
		return structures.Success, "", nil
	} else {
		msg := fmt.Sprintf("Got %v as attested value but expected %v", claimedAV, expectedAV)
		return structures.Fail, msg, nil
	}
}

func FirmwareRule(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)

	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	claimedFirmware := fmt.Sprintf("%v", q.FirmwareVersion)
	expectedFirmware := (ev.EVS)["firmwareVersion"]

	if expectedFirmware == claimedFirmware {
		return structures.Success, "Firmware version matches", nil
	} else {
		msg := fmt.Sprintf("Got %v as firmware version but expected %v", claimedFirmware, expectedFirmware)
		return structures.Fail, msg, nil
	}

}

func MagicNumberRule(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	if q.Magic != "ff544347" {
		msg := fmt.Sprintf("TPM Quote (TPMS_ATTEST) type value is wrong - probably not a quote - received %v, expected ff544347", q.Magic)
		return structures.Fail, msg, nil
	}

	return structures.Success, "", nil
}

func QuoteTypeRule(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	if q.Type != "8018" {
		msg := fmt.Sprintf("TPM Quote (TPMS_ATTEST) type value is wrong - probably not a quote - received %v, expected 8018", q.Type)
		return structures.Fail, msg, nil
	}

	return structures.Success, "TPM Quote (TPMS_ATTEST) type value is correct", nil
}

func ValidNonce(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	quote, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}
	quoteNonceValue := quote.ExtraData

	claimNonceValue := fmt.Sprintf("%s", claim.Header.CallParameters["tpm2/nonce"])

	if claimNonceValue != quoteNonceValue {
		return structures.Fail, fmt.Sprintf("Nonce are not matching, got: %v, expected: %v", quoteNonceValue, claimNonceValue), nil
	}

	msg := fmt.Sprintf("Nonce matches: (base64) %v", quoteNonceValue)
	return structures.Success, msg, nil
}

// Constructs AttestableData struct with signature
// TODO find way to cache this in the session object

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

func getQuote(claim structures.Claim) (quoteStructure, error) {
	quoteData, ok := (claim.Body)["quote"]
	//fmt.Printf("\n### GetQuote %v\n%v\n%v\n\n", reflect.TypeOf(quoteData), ok, quoteData)
	if !ok {
		return quoteStructure{}, fmt.Errorf("claim does not contain quote")

	}

	var qs quoteStructure
	mapstructure.Decode(quoteData, &qs)
	return qs, nil
}
