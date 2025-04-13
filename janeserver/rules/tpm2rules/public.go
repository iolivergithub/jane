package tpm2rules

import (
	"encoding/base64"
	"fmt"
	"reflect"

	"a10/structures"

	"github.com/mitchellh/mapstructure" // from Hasicorp https://stackoverflow.com/questions/26744873/converting-map-to-struct
)

func Registration() []structures.Rule {
	attestedPCRDigest := structures.Rule{"tpm2_attestedValue", "Checks the TPM's reported attested value against the expected value", AttestedPCRDigest, true}
	//checkPCRSelection := structures.Rule{"tpm2_PCRSelection", "Checks if a quote includes the correct PCRs", checkPCRSelection, true}
	//checkQuoteDigest256 := structures.Rule{"tpm2_quoteDigest256", "Checks if a claim of PCRs match the hash in the quote (sha256)", checkQuoteDigest256, false}
	ruleFirmware := structures.Rule{"tpm2_firmware", "Checks the TPM firmware version against the expected value", FirmwareRule, true}
	ruleMagic := structures.Rule{"tpm2_magicNumber", "Checks the quote magic number is 0xFF544347", MagicNumberRule, false}
	ruleQuoteType := structures.Rule{"tpm2_type", "Checks the type of the quote which must be 0x8018", QuoteTypeRule, false}
	ruleIsSafe := structures.Rule{"tpm2_safe", "Checks that the value of safe is 1", IsSafe, false}
	//ruleValidSignature := structures.Rule{"tpm2_validSignature", "Checks that the signature of rule is valid against the signing attestation key", ValidSignature, false}
	ruleValidNonce := structures.Rule{"tpm2_validNonce", "Checks that nonce used for the claim matches the nonce in the quote", ValidNonce, false}

	return []structures.Rule{ruleFirmware, ruleMagic, attestedPCRDigest, ruleIsSafe, ruleValidNonce, ruleQuoteType}
}

func IsSafe(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	if q.ClockInfo.Safe == "false" {
		return structures.Fail, "Uncommanded device/TPM shutdown", nil
	}

	return structures.Success, "TPM shutdown normal", nil
}

func AttestedPCRDigest(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	pcrdigest := []byte(q.Attested.PCRDigest)

	// THIS IS THE OLD LINE: DO NOT USE
	//claimedAV := hex.EncodeToString(quoteData.PCRDigest.Buffer)

	//looks like Thore in his utilities/tpm2.go encoded the PCRDigest as Base64...so further doing this with
	// hex to string of that base64 is overkill and complicates the expected values
	// which if the above claimedAV line is written means that we have to write the expectedvalue for a PCRDigest
	// in hex, but we see only base64 in the claim

	//claimedAV := fmt.Sprintf("%v",quoteData.PCRDigest.Buffer)
	//claimedAV := string(quoteData.PCRDigest.Buffer[:])
	claimedAV := base64.StdEncoding.EncodeToString(pcrdigest)
	expectedAV := (ev.EVS)["attestedValue"]

	fmt.Printf("\n\nPCRDigest %v \n claimedAV %v \nexpectedAV %v\n\n", pcrdigest, claimedAV, expectedAV)

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

	fmt.Printf("GOT HERE WITH A GOOD QUOTE; ERR=%v\n", err)
	fmt.Printf("Quote looks like this: \n %v \n\n", q)

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

// func ValidSignature(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
// 	quote, err := getQuote(claim)
// 	if err != nil {
// 		return structures.Fail, "Parsing TPM quote failed", err
// 	}

// 	akBytes, err := base64.StdEncoding.DecodeString(claim.Header.Element.TPM2.AK.Public)
// 	if err != nil {
// 		return structures.RuleCallFailure, "Base64 decoding the AK failed", err
// 	}
// 	akKey, err := utilities.ParseTPMKey(akBytes)
// 	if err != nil {
// 		return structures.Fail, "Parsing AK failed", err
// 	}

// 	if err := quote.VerifySignature(akKey); err != nil {
// 		return structures.Fail, "Validation of the quote failed", nil
// 	}

// 	return structures.Success, "Quote was validated successfully", nil
// }

func ValidNonce(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	quote, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}
	quoteNonceValue := quote.ExtraData

	claimNonceValue := fmt.Sprintf("%s", claim.Header.CallParameters["tpm2/nonce"])
	// if !ok {
	// 	return structures.RuleCallFailure, "claim has no nonce", nil
	// }

	//fmt.Println("***\nNonce are not matching, got: %v, expected: %v, base64:", quoteNonceValue, claimNonceValue, base64.StdEncoding.EncodeToString(quoteNonceValue))

	if claimNonceValue != quoteNonceValue {
		return structures.Fail, fmt.Sprintf("Nonce are not matching, got: %v, expected: %v", quoteNonceValue, claimNonceValue), nil
	}

	msg := fmt.Sprintf("Nonce matches: (base64) %v", quoteNonceValue)
	return structures.Success, msg, nil
}

// func checkPCRSelection(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
// 	quote, err := getQuote(claim)
// 	if err != nil {
// 		return structures.Fail, "Parsing TPM quote failed", err
// 	}

// 	pcrdigest := quote.Attested.PCRDigest
// 	if err != nil {
// 		return structures.Fail, "Parsing Attest structure into Quote failed", err
// 	}
// 	selection := utilities.TPMSPCRSelectionToList(data.PCRSelect.PCRSelections)

// 	evSelection, ok := ev.EVS["pcrselection"]
// 	if !ok {
// 		return structures.MissingExpectedValue, "pcrselection not given", err
// 	}
// 	var evSelectionList []int
// 	for _, v := range evSelection.(primitive.A) {
// 		index, err := strconv.Atoi(v.(string))
// 		if err != nil {
// 			return structures.Fail, "pcrselection contains non integer strings", err
// 		}
// 		evSelectionList = append(evSelectionList, index)
// 	}
// 	if len(evSelectionList) != len(selection) {
// 		return structures.Fail, "not the same length", err
// 	}
// 	for _, v := range evSelectionList {
// 		if !slices.Contains(selection, v) {
// 			return structures.Fail, fmt.Sprintf("Index %d is missing in quote", v), err
// 		}
// 	}
// 	return structures.Success, "", err
// }

// func checkQuoteDigest256(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
// 	quote, err := getQuote(claim)
// 	if err != nil {
// 		return structures.Fail, "Parsing TPM quote failed", err
// 	}
// 	pcrsClaimID := parameter["pcrscid"].(string)

// 	pcrsClaim, err := operations.GetClaimByItemID(pcrsClaimID)
// 	if err != nil {
// 		return structures.Fail, "Could not get PCRs claim", err
// 	}

// 	data, err := quote.Attested.Quote()
// 	if err != nil {
// 		return structures.Fail, "Parsing Attest structure into Quote failed", err
// 	}
// 	digest := data.PCRDigest.Buffer
// 	selection := utilities.TPMSPCRSelectionToList(data.PCRSelect.PCRSelections)

// 	sha256Entries := make(map[string]string)
// 	for k, v := range pcrsClaim.Body["sha256"].(map[string]interface{}) {
// 		sha256Entries[k] = v.(string)
// 	}

// 	hash := sha256.New()
// 	for _, pcrIndex := range selection {
// 		pcrIndexS := fmt.Sprintf("%d", pcrIndex)

// 		entry, ok := sha256Entries[pcrIndexS]
// 		if !ok {
// 			return structures.Fail, fmt.Sprintf("PCR index missing in PCR claim: %s", entry), err
// 		}
// 		entryBytes, err := hex.DecodeString(entry)
// 		if err != nil {
// 			return structures.Fail, "Entry not valid hex", err
// 		}
// 		hash.Write(entryBytes)
// 	}

// 	digestPCRs := hash.Sum([]byte{})
// 	if !bytes.Equal(digestPCRs, digest) {
// 		return structures.Fail, "PCRs and hash in quote do not match", err
// 	}

// 	return structures.Success, "PCRs and hash in quote match", err
// }

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
	fmt.Printf("\n### GetQuote %v\n%v\n%v\n\n", reflect.TypeOf(quoteData), ok, quoteData)
	if !ok {
		return quoteStructure{}, fmt.Errorf("claim does not contain quote")

	}

	var qs quoteStructure
	mapstructure.Decode(quoteData, &qs)
	return qs, nil
}
