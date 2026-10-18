package tpm2rules

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"a10/structures"
)

// PCRDigest checks that the PCR values in the tpm2/pcrs claim given by the
// rule parameter pcrscid are the ones this quote signed: it hashes the
// selected PCRs and compares the result with the quote's attested value
// (#70). The PCRs claim is unsigned; this is what ties it to a signed quote.
func PCRDigest(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	if f := claim.Header.Intent.Function; f != "tpm2/quote" {
		return structures.Fail, fmt.Sprintf("This rule runs on a tpm2/quote claim, not %s", f), nil
	}
	q, err := getQuote(claim)
	if err != nil {
		return structures.Fail, "Parsing TPM quote failed", err
	}

	// The quote body's pcrselect is not parseable; the bank and selection
	// are taken from the call parameters the quote was made with.
	bank, _ := claim.Header.CallParameters["bank"].(string)
	sel, _ := claim.Header.CallParameters["pcrSelection"].(string)
	if bank == "" || sel == "" {
		return structures.Fail, "The quote claim has no bank or pcrSelection call parameter", nil
	}
	pcrs, err := parsePCRSelection(sel)
	if err != nil {
		return structures.Fail, err.Error(), nil
	}

	pcrsClaim, msg := linkedClaim(claim, parameter, "pcrscid", "tpm2/pcrs", "tpm2/newpcrs")
	if msg != "" {
		return structures.Fail, msg, nil
	}
	values, ok := pcrsClaim.Body[bank].(map[string]interface{})
	if !ok {
		return structures.Fail, fmt.Sprintf("The PCRs claim has no %s bank", bank), nil
	}

	// The TPM concatenates the selected PCRs in ascending index order.
	h := sha256.New() // tarzan's AKs sign quotes with RSASSA/SHA-256
	for _, i := range pcrs {
		s, ok := values[strconv.Itoa(i)].(string)
		if !ok {
			return structures.Fail, fmt.Sprintf("PCR %d of bank %s is not in the PCRs claim", i, bank), nil
		}
		d, err := hex.DecodeString(s)
		if err != nil {
			return structures.Fail, fmt.Sprintf("PCR %d of bank %s in the PCRs claim is not hex", i, bank), nil
		}
		h.Write(d)
	}

	want, err := base64.StdEncoding.DecodeString(q.Attested.PCRDigest)
	if err != nil {
		return structures.Fail, "The quote's attested value (pcrdigest) is not base64", nil
	}
	gap := sessionGap(claim, pcrsClaim)
	if !bytes.Equal(h.Sum(nil), want) {
		return structures.Fail, fmt.Sprintf("PCRs %v of bank %s in the PCRs claim do not give the quote's attested value. "+
			"A PCR extended between the two claims (typically PCR 10 with IMA) gives a mismatch without any tampering; "+
			"collect the PCRs and the quote in the same session.%s", pcrs, bank, gap), nil
	}
	return structures.Success, fmt.Sprintf("PCRs %v of bank %s in the PCRs claim give the quote's attested value.%s", pcrs, bank, gap), nil
}

// parsePCRSelection turns a selection such as "4,5,0" into sorted, unique
// PCR indices.
func parsePCRSelection(sel string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, f := range strings.Split(sel, ",") {
		f = strings.TrimSpace(f)
		i, err := strconv.Atoi(f)
		if err != nil || i < 0 || i > 23 {
			return nil, fmt.Errorf("pcrSelection %q contains %q, which is not a PCR index", sel, f)
		}
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	sort.Ints(out)
	return out, nil
}
