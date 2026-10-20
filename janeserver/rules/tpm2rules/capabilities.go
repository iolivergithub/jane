package tpm2rules

import (
	"fmt"
	"strconv"

	"a10/structures"
)

// CapabilitiesFirmware runs on a tpm2/capabilities claim and checks the TPM
// firmware version it reports equals the one in the quote or gettime claim
// named by the rule parameter quotecid (same element) (#77). The capabilities
// claim is unsigned; the quote's firmware version is taken from its signed
// attest, so this ties the reported TPM identity to the signing TPM, at least
// as far as the firmware goes. The rule does not check the quote's signature
// itself: run tpm2_validSignature on that claim too.
func CapabilitiesFirmware(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	if f := claim.Header.Intent.Function; f != "tpm2/capabilities" {
		return structures.Fail, fmt.Sprintf("Rule applies to tpm2/capabilities claims, not %s", f), nil
	}
	summary, _ := claim.Body["summary"].(map[string]interface{})
	capsFW, _ := summary["firmwareversion"].(string)
	if capsFW == "" {
		return structures.Fail, "Capabilities claim has no summary.firmwareversion", nil
	}
	// tarzan writes this as %08x%08x (FIRMWARE_VERSION_1, _2); compare as numbers.
	capsN, err := strconv.ParseUint(capsFW, 16, 64)
	if err != nil {
		return structures.Fail, fmt.Sprintf("summary.firmwareversion %q is not a hex number", capsFW), nil
	}

	quote, msg := linkedClaim(claim, parameter, "quotecid", "tpm2/quote", "tpm2/gettime")
	if msg != "" {
		return structures.Fail, msg, nil
	}
	qv, msg := claimClock(quote)
	if msg != "" {
		return structures.Fail, fmt.Sprintf("Claim %s given in quotecid: %s", quote.ItemID, msg), nil
	}
	from := "signed attest"
	if !qv.FromAttest {
		from = "decoded quote (no attest in that claim)"
	}

	if capsN != qv.Firmware {
		return structures.Fail, fmt.Sprintf("Firmware versions differ: capabilities report %016x, the %s claim %s (%s) has %016x.%s",
			capsN, quote.Header.Intent.Function, quote.ItemID, from, qv.Firmware, sessionGap(claim, quote)), nil
	}
	return structures.Success, fmt.Sprintf("Firmware version %016x matches the %s claim %s (%s).%s",
		capsN, quote.Header.Intent.Function, quote.ItemID, from, sessionGap(claim, quote)), nil
}
