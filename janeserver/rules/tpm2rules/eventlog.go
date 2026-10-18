package tpm2rules

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"

	"a10/structures"
	"a10/utilities"
)

// pcrBankNames are the bank names tarzan uses in tpm2/pcrs claims.
var pcrBankNames = map[string]bool{"sha1": true, "sha256": true, "sha384": true, "sha512": true}

// UEFILogReplay replays the UEFI event log in the uefi/eventlog claim given
// by the rule parameter uefilogcid and checks the result against the PCR
// values of this tpm2/pcrs claim, per bank (#71). Only the PCRs the log
// extends are compared; a bank in only one of the two claims is skipped and
// mentioned. It shows the log is consistent with the PCRs, not that the
// events themselves are good.
func UEFILogReplay(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	if f := claim.Header.Intent.Function; f != "tpm2/pcrs" && f != "tpm2/newpcrs" {
		return structures.Fail, fmt.Sprintf("This rule runs on a tpm2/pcrs claim, not %s", f), nil
	}
	logClaim, msg := linkedClaim(claim, parameter, "uefilogcid", "uefi/eventlog")
	if msg != "" {
		return structures.Fail, msg, nil
	}
	b64, _ := logClaim.Body["eventlog"].(string)
	el, err := utilities.DecodeEventLog(b64)
	if err != nil {
		return structures.Fail, "The event log could not be parsed: " + err.Error(), nil
	}

	logBanks := utilities.EventLogBanks(el)
	extended := extendedPCRs(el)

	var report []string
	compared, failed := 0, false
	for _, name := range sortedKeys(logBanks) {
		values, ok := claim.Body[name].(map[string]interface{})
		if !ok {
			report = append(report, name+": not in the PCRs claim, skipped")
			continue
		}
		alg := logBanks[name]

		var mrs []register.MR
		var missing []int
		for _, i := range extended {
			s, ok := values[strconv.Itoa(i)].(string)
			if !ok {
				missing = append(missing, i)
				continue
			}
			d, err := hex.DecodeString(s)
			if err != nil {
				return structures.Fail, fmt.Sprintf("PCR %d of bank %s in the PCRs claim is not hex", i, name), nil
			}
			mrs = append(mrs, register.PCR{Index: i, Digest: d, DigestAlg: alg.CryptoHash()})
		}
		if len(mrs) == 0 {
			report = append(report, name+": none of the PCRs the log extends are in the PCRs claim, skipped")
			continue
		}
		compared++

		line := name + ": "
		_, err := el.Verify(mrs)
		var replayErr tcg.ReplayError
		switch {
		case err == nil:
			line += fmt.Sprintf("PCRs %v match", indices(mrs))
		case errors.As(err, &replayErr):
			failed = true
			bad := append([]int(nil), replayErr.InvalidMRs...)
			sort.Ints(bad)
			line += fmt.Sprintf("PCRs %v do NOT match (of %v)", bad, indices(mrs))
		default:
			failed = true
			line += "replay failed: " + err.Error()
		}
		if len(missing) > 0 {
			line += fmt.Sprintf("; PCRs %v not in the PCRs claim", missing)
		}
		report = append(report, line)
	}
	for _, name := range sortedKeys(claim.Body) {
		if _, inLog := logBanks[name]; pcrBankNames[name] && !inLog {
			report = append(report, name+": not in the event log, skipped")
		}
	}

	msg = strings.Join(report, ". ") + "." + sessionGap(claim, logClaim)
	if compared == 0 {
		return structures.Fail, "No bank to compare. " + msg, nil
	}
	if failed {
		return structures.Fail, "The event log does not replay to the PCR values. " + msg, nil
	}
	return structures.Success, "The event log replays to the PCR values. " + msg, nil
}

// extendedPCRs lists the PCRs the log extends (events other than
// EV_NO_ACTION, which are not extended).
func extendedPCRs(el *tcg.EventLog) []int {
	seen := map[int]bool{}
	for _, alg := range el.Algs {
		for _, e := range el.Events(alg) {
			if e.Type.String() != "EV_NO_ACTION" {
				seen[e.Index] = true
			}
		}
		break // the PCR of an event is the same in every bank
	}
	var out []int
	for i := range seen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

func indices(mrs []register.MR) []int {
	out := make([]int, len(mrs))
	for i, m := range mrs {
		out[i] = m.Idx()
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
