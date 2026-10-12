// Rules for claims produced by the RATSD protocol's ratsd/chares intent.
//
// The body of such a claim is the legacy RATSD token returned by ratsd:
//
//	{
//	  "eat_profile": "tag:github.com,2024:veraison/ratsd",
//	  "eat_nonce":   "<the request nonce, standard base64 with padding>",
//	  "vnd.veraison.nonce_adjust_function": "shake-256",
//	  "vnd.veraison.nonce_adjust_map": { "<attester>": <size>, ... },
//	  "cmw": "<standard base64 of the CMW collection>"
//	}
//
// and the nonce Jane sent is recorded in the claim's call parameters as ratsd/nonce
// (base64url without padding).
package chares

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"a10/structures"
)

func Registration() []structures.Rule {
	ruleValidNonce := structures.Rule{"chares_validNonce", "Checks that the eat_nonce returned by ratsd matches the nonce sent in the ratsd/chares call", ValidNonce, false}
	ruleAttesterNonces := structures.Rule{"chares_validAttesterNonces", "Checks that each leaf attester's evidence contains the nonce ratsd derived for it from the nonce sent in the ratsd/chares call", ValidAttesterNonces, false}

	return []structures.Rule{ruleValidNonce, ruleAttesterNonces}
}

// ValidNonce compares the eat_nonce in the RATSD token with the ratsd/nonce call parameter.
//
// The two are compared as bytes, not strings: Jane sends the nonce as base64url without padding
// but ratsd returns it as standard base64 with padding, so the strings normally differ.
func ValidNonce(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	eatNonce, err := getEatNonce(claim)
	if err != nil {
		return structures.Fail, "Parsing RATSD token failed", err
	}

	sent, ok := claim.Header.CallParameters["ratsd/nonce"]
	if !ok {
		return structures.Fail, "Claim has no ratsd/nonce call parameter", fmt.Errorf("claim call parameters do not contain ratsd/nonce")
	}
	sentNonce := fmt.Sprintf("%v", sent)

	eatNonceBytes, err := decodeBase64(eatNonce)
	if err != nil {
		return structures.Fail, fmt.Sprintf("eat_nonce %v is not base64: %v", eatNonce, err), nil
	}

	sentNonceBytes, err := decodeBase64(sentNonce)
	if err != nil {
		return structures.Fail, fmt.Sprintf("ratsd/nonce %v is not base64: %v", sentNonce, err), nil
	}

	if !bytes.Equal(eatNonceBytes, sentNonceBytes) {
		return structures.Fail, fmt.Sprintf("Nonce are not matching, got: %v, expected: %v", eatNonce, sentNonce), nil
	}

	msg := fmt.Sprintf("Nonce matches: (%d bytes) %v", len(sentNonceBytes), sentNonce)
	return structures.Success, msg, nil
}

// getEatNonce returns the eat_nonce from the RATSD token in the claim body.
// EAT allows an array of nonces; ratsd only ever sends one, so anything else is an error.
func getEatNonce(claim structures.Claim) (string, error) {
	n, ok := (claim.Body)["eat_nonce"]
	if !ok {
		return "", fmt.Errorf("claim does not contain eat_nonce")
	}

	s, ok := n.(string)
	if !ok {
		return "", fmt.Errorf("eat_nonce is not a single string: %v", n)
	}

	return s, nil
}

// decodeBase64 accepts standard or URL-safe base64, with or without padding.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}

	if strings.ContainsAny(s, "-_") {
		return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	}

	return base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
}
