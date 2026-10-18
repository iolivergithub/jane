package tpm2rules

import (
	"fmt"
	"time"

	"a10/operations"
	"a10/structures"
)

// Helpers for rules that check one claim against another claim given by its
// itemid in a rule parameter (#70, #71).

// getClaim fetches a claim by itemid; replaced in tests.
var getClaim = operations.GetClaimByItemID

// linkedClaim fetches the claim named by the rule parameter param and checks
// it is a usable claim of one of the given intent functions for the same
// element. On failure it returns the reason as a non-empty message.
func linkedClaim(claim structures.Claim, parameter map[string]interface{}, param string, functions ...string) (structures.Claim, string) {
	cid, _ := parameter[param].(string)
	if cid == "" {
		return structures.Claim{}, fmt.Sprintf("Rule parameter %s (a claim itemid) is missing", param)
	}
	other, err := getClaim(cid)
	if err != nil {
		return structures.Claim{}, fmt.Sprintf("Claim %s given in %s could not be read: %v", cid, param, err)
	}
	if other.BodyType == structures.CLAIMERROR {
		return structures.Claim{}, fmt.Sprintf("Claim %s given in %s is an error claim", cid, param)
	}
	ok := false
	for _, f := range functions {
		if other.Header.Intent.Function == f {
			ok = true
		}
	}
	if !ok {
		return structures.Claim{}, fmt.Sprintf("Claim %s given in %s is a %s claim, expected %v", cid, param, other.Header.Intent.Function, functions)
	}
	if other.Header.Element.ItemID != claim.Header.Element.ItemID {
		return structures.Claim{}, fmt.Sprintf("Claim %s given in %s is for another element (%s)", cid, param, other.Header.Element.ItemID)
	}
	return other, ""
}

// sessionGap says how far apart two claims were taken when they are from
// different sessions, and is empty when they are from the same session.
func sessionGap(a, b structures.Claim) string {
	if a.Header.Session.ItemID == b.Header.Session.ItemID {
		return ""
	}
	d := time.Duration(a.Header.Timing.Requested - b.Header.Timing.Requested)
	if d < 0 {
		d = -d
	}
	return fmt.Sprintf(" Note: the claims are from different sessions, taken %v apart.", d.Round(time.Second))
}
