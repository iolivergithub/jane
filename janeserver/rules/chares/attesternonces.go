package chares

// chares_validAttesterNonces
//
// ratsd does not pass the request nonce to its leaf attesters. For each attester it derives
//
//	SHAKE(nonce)[:size]
//
// using the function named in vnd.veraison.nonce_adjust_function and the size recorded for that
// attester in vnd.veraison.nonce_adjust_map, and the attester binds that value into its evidence.
// This rule recomputes the derived nonce from the ratsd/nonce Jane sent and looks for it in each
// attester's evidence inside the CMW collection.
//
// The CMW collection (claim body "cmw", standard base64) decodes to
//
//	{ "__cmwc_t": "...", "<attester>": [ "<media type>", "<base64url evidence>" ], ... }
//
// Supported evidence media types:
//
//	application/vnd.veraison.tsm-report+json         {"provider", "outblob" (base64url), ...}
//	    The derived nonce must appear in outblob, either as raw bytes (the TEE's report data, e.g.
//	    TDX, SEV-SNP) or as lower case hex (the fake TSM used by ratsd's mock-tsm attester).
//	application/vnd.veraison.nvidia-gpu-evidence+json {"devices": [{"nonce", "evidence", ...}]}
//	    For every device, the nonce field must equal the derived nonce and the attestation report
//	    must contain it.

import (
	"bytes"
	"crypto/sha3"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"a10/structures"
)

const (
	nonceAdjustFunctionClaim = "vnd.veraison.nonce_adjust_function"
	nonceAdjustMapClaim      = "vnd.veraison.nonce_adjust_map"
	cmwCollectionTypeKey     = "__cmwc_t"

	tsmReportJSON   = "application/vnd.veraison.tsm-report+json"
	gpuEvidenceJSON = "application/vnd.veraison.nvidia-gpu-evidence+json"
)

func ValidAttesterNonces(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {

	// The nonce Jane sent
	sent, ok := claim.Header.CallParameters["ratsd/nonce"]
	if !ok {
		return structures.Fail, "Claim has no ratsd/nonce call parameter", fmt.Errorf("claim call parameters do not contain ratsd/nonce")
	}
	sentNonce, err := decodeBase64(fmt.Sprintf("%v", sent))
	if err != nil {
		return structures.Fail, fmt.Sprintf("ratsd/nonce %v is not base64: %v", sent, err), nil
	}

	// How ratsd adjusted it
	adjustFn, adjustMap, err := getNonceAdjustment(claim)
	if err != nil {
		return structures.Fail, "Parsing RATSD token failed", err
	}

	// The evidence
	collection, err := getCMWCollection(claim)
	if err != nil {
		return structures.Fail, "Parsing RATSD token failed", err
	}

	attesters := make([]string, 0, len(collection))
	for k := range collection {
		if k != cmwCollectionTypeKey {
			attesters = append(attesters, k)
		}
	}
	sort.Strings(attesters)

	if len(attesters) == 0 {
		return structures.Fail, "The CMW collection contains no attester evidence", nil
	}

	passed := []string{}
	failed := []string{}

	for _, a := range attesters {
		size, ok := adjustMap[a]
		if !ok {
			failed = append(failed, fmt.Sprintf("%v: no entry in %v", a, nonceAdjustMapClaim))
			continue
		}

		expected, err := deriveNonce(adjustFn, sentNonce, size)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%v: %v", a, err))
			continue
		}

		where, err := checkEvidence(collection[a], expected)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%v: %v", a, err))
			continue
		}

		passed = append(passed, fmt.Sprintf("%v: %d-byte nonce %v", a, size, where))
	}

	if len(failed) > 0 {
		msg := "Attester nonce check failed. " + strings.Join(failed, "; ")
		if len(passed) > 0 {
			msg = msg + ". Passed: " + strings.Join(passed, "; ")
		}
		return structures.Fail, msg, nil
	}

	return structures.Success, fmt.Sprintf("All %d attester nonces match (%v). %v", len(passed), adjustFn, strings.Join(passed, "; ")), nil
}

// getNonceAdjustment returns the adjustment function and the per-attester nonce sizes.
// The map is round-tripped through JSON so that it reads the same whether the claim body came
// straight from ratsd or back from the database.
func getNonceAdjustment(claim structures.Claim) (string, map[string]int, error) {
	fn, ok := (claim.Body)[nonceAdjustFunctionClaim]
	if !ok {
		return "", nil, fmt.Errorf("claim does not contain %v", nonceAdjustFunctionClaim)
	}
	adjustFn, ok := fn.(string)
	if !ok {
		return "", nil, fmt.Errorf("%v is not a string: %v", nonceAdjustFunctionClaim, fn)
	}

	m, ok := (claim.Body)[nonceAdjustMapClaim]
	if !ok {
		return "", nil, fmt.Errorf("claim does not contain %v", nonceAdjustMapClaim)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", nil, fmt.Errorf("%v could not be read: %w", nonceAdjustMapClaim, err)
	}
	adjustMap := make(map[string]int)
	if err := json.Unmarshal(raw, &adjustMap); err != nil {
		return "", nil, fmt.Errorf("%v is not a map of attester names to sizes: %w", nonceAdjustMapClaim, err)
	}

	return adjustFn, adjustMap, nil
}

// getCMWCollection decodes the cmw claim into its records, keyed by attester name
func getCMWCollection(claim structures.Claim) (map[string]json.RawMessage, error) {
	c, ok := (claim.Body)["cmw"]
	if !ok {
		return nil, fmt.Errorf("claim does not contain cmw")
	}
	s, ok := c.(string)
	if !ok {
		return nil, fmt.Errorf("cmw is not a string")
	}

	decoded, err := decodeBase64(s)
	if err != nil {
		return nil, fmt.Errorf("cmw is not base64: %w", err)
	}

	collection := make(map[string]json.RawMessage)
	if err := json.Unmarshal(decoded, &collection); err != nil {
		return nil, fmt.Errorf("cmw is not a JSON CMW collection: %w", err)
	}

	return collection, nil
}

// deriveNonce computes the nonce ratsd gave to an attester
func deriveNonce(fn string, nonce []byte, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid nonce size %d", size)
	}

	switch fn {
	case "shake-256":
		return sha3.SumSHAKE256(nonce, size), nil
	case "shake-128":
		return sha3.SumSHAKE128(nonce, size), nil
	default:
		return nil, fmt.Errorf("unsupported nonce adjustment function %q", fn)
	}
}

// checkEvidence looks for the expected nonce in one CMW record, [media type, base64url value, ...]
// and returns where it was found
func checkEvidence(record json.RawMessage, expected []byte) (string, error) {
	var fields []interface{}
	if err := json.Unmarshal(record, &fields); err != nil || len(fields) < 2 {
		return "", fmt.Errorf("CMW entry is not a [type, value] record")
	}
	mediaType, ok1 := fields[0].(string)
	value, ok2 := fields[1].(string)
	if !ok1 || !ok2 {
		return "", fmt.Errorf("CMW entry is not a [type, value] record")
	}

	evidence, err := decodeBase64(value)
	if err != nil {
		return "", fmt.Errorf("evidence is not base64url: %w", err)
	}

	// media types may carry parameters
	switch strings.TrimSpace(strings.SplitN(mediaType, ";", 2)[0]) {
	case tsmReportJSON:
		return checkTSMReport(evidence, expected)
	case gpuEvidenceJSON:
		return checkGPUEvidence(evidence, expected)
	default:
		return "", fmt.Errorf("unsupported evidence type %v", mediaType)
	}
}

func checkTSMReport(evidence []byte, expected []byte) (string, error) {
	var report struct {
		Provider string `json:"provider"`
		OutBlob  string `json:"outblob"`
	}
	if err := json.Unmarshal(evidence, &report); err != nil {
		return "", fmt.Errorf("TSM report is not JSON: %w", err)
	}
	report.Provider = strings.TrimSpace(report.Provider)

	outblob, err := decodeBase64(report.OutBlob)
	if err != nil {
		return "", fmt.Errorf("TSM report outblob is not base64url: %w", err)
	}

	if bytes.Contains(outblob, expected) {
		return fmt.Sprintf("found in %v report outblob", report.Provider), nil
	}
	if bytes.Contains(outblob, []byte(hex.EncodeToString(expected))) {
		return fmt.Sprintf("found as hex in %v report outblob", report.Provider), nil
	}

	return "", fmt.Errorf("derived nonce %v not found in %v report outblob", base64.RawURLEncoding.EncodeToString(expected), report.Provider)
}

func checkGPUEvidence(evidence []byte, expected []byte) (string, error) {
	// []byte fields are standard base64 in Go's JSON encoding
	var gpu struct {
		Devices []struct {
			Nonce    []byte `json:"nonce"`
			Arch     string `json:"arch"`
			Evidence []byte `json:"evidence"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(evidence, &gpu); err != nil {
		return "", fmt.Errorf("GPU evidence is not JSON: %w", err)
	}
	if len(gpu.Devices) == 0 {
		return "", fmt.Errorf("GPU evidence contains no devices")
	}

	for i, d := range gpu.Devices {
		if !bytes.Equal(d.Nonce, expected) {
			return "", fmt.Errorf("GPU device %d (%v) nonce does not match the derived nonce", i, d.Arch)
		}
		if !bytes.Contains(d.Evidence, expected) {
			return "", fmt.Errorf("GPU device %d (%v) attestation report does not contain the derived nonce", i, d.Arch)
		}
	}

	return fmt.Sprintf("found in the attestation report of %d GPU device(s)", len(gpu.Devices)), nil
}
