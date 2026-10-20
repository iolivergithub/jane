package tpm2rules

import (
	"fmt"
	"time"

	"a10/structures"
)

// ClockProgress runs on a tpm2/quote or tpm2/gettime claim and compares the
// TPM's clock, reset and restart counts with an earlier quote or gettime
// claim of the same element, named by the compulsory rule parameter
// previouscid. There is deliberately no automatic search for a previous claim.
//
// Fails if the reset count went down, if the restart count went down with
// the same reset count (TPM cleared, or a different TPM), or if the clock went
// backwards (whether or not the TPM reports safe). Otherwise it passes and
// reports the reboots and restarts in between.
func ClockProgress(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	if _, msg := signedFunction(claim); msg != "" {
		return structures.Fail, msg, nil
	}
	prev, msg := linkedClaim(claim, parameter, "previouscid", "tpm2/quote", "tpm2/gettime")
	if msg != "" {
		return structures.Fail, msg, nil
	}
	if prev.Header.Timing.Requested >= claim.Header.Timing.Requested {
		return structures.Fail, fmt.Sprintf("Claim %s given in previouscid is not earlier than this claim", prev.ItemID), nil
	}

	now, msg := claimClock(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	was, msg := claimClock(prev)
	if msg != "" {
		return structures.Fail, fmt.Sprintf("Claim %s given in previouscid: %s", prev.ItemID, msg), nil
	}
	note := sessionGap(claim, prev)
	if !now.FromAttest || !was.FromAttest {
		note += " Note: at least one claim has no signed attest; its decoded clock values were used."
	}

	switch {
	case now.ResetCount < was.ResetCount:
		return structures.Fail, fmt.Sprintf("TPM reset count went from %d to %d: TPM cleared or a different TPM.%s", was.ResetCount, now.ResetCount, note), nil
	case now.ResetCount == was.ResetCount && now.RestartCount < was.RestartCount:
		return structures.Fail, fmt.Sprintf("TPM restart count went from %d to %d with no reset: TPM cleared or a different TPM.%s", was.RestartCount, now.RestartCount, note), nil
	case now.Clock < was.Clock && now.Safe:
		return structures.Fail, fmt.Sprintf("TPM clock went backwards (%d -> %d ms) although the TPM reports safe.%s", was.Clock, now.Clock, note), nil
	case now.Clock < was.Clock:
		return structures.Fail, fmt.Sprintf("TPM clock went backwards (%d -> %d ms) after an unclean shutdown (safe = false): the TPM clock was not saved before power was lost.%s", was.Clock, now.Clock, note), nil
	}

	resets := now.ResetCount - was.ResetCount
	restarts := now.RestartCount
	if resets == 0 {
		restarts = now.RestartCount - was.RestartCount
	}
	advanced := time.Duration(now.Clock-was.Clock) * time.Millisecond
	return structures.Success, fmt.Sprintf("Since claim %s: %d reset(s) (reboots), %d restart(s)%s; TPM clock advanced by %v.%s",
		prev.ItemID, resets, restarts, map[bool]string{true: " since the last reset", false: ""}[resets > 0], advanced.Round(time.Millisecond), note), nil
}
