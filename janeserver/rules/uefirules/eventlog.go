package uefirules

// Rules on the measured UEFI state in a uefi/eventlog claim (#134): what the
// firmware measured about Secure Boot into PCR 7 and its version into PCR 0.
// The UEFI_ rules check what the running OS reports (uefi/efivars); these
// check what was measured at boot. They read the log as it is: their trust
// comes from tpm2_uefiLogReplay passing on the same log claim, against a
// quote, in the same run.
//
// uefi_logMatchesEfivars runs on a uefi/efivars claim instead, with the log
// claim as parameter uefilogcid, and needs no expected value. The others
// read one expected value (written by jp's
// type uefieventlog), one entry per rule:
//
//	"secureboot":      "enabled" | "disabled"
//	"pk", "kek", "db", "dbx": "sha256:<hex>" of the measured data, or "absent"
//	"authorities":     ["sha256:<hex>", ...] certificates allowed to verify boot images
//	"dmaprotection":   "notreported" | "disabled"
//	"firmwareversion": the S-CRTM version string
//
// An entry missing from the expected value fails, as for the UEFI_ rules.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"a10/operations"
	"a10/structures"
	"a10/utilities"

	"github.com/iolivergithub/jane/lib/uefi"
)

// getClaim fetches a claim by itemid; replaced in tests.
var getClaim = operations.GetClaimByItemID

func logRegistration() []structures.Rule {
	rule := func(name, desc string, f structures.RuleCall, ev bool) structures.Rule {
		return structures.Rule{Name: name, Description: desc, CallFunction: f, NeedsEV: ev}
	}
	rules := []structures.Rule{
		rule("uefi_logSecureBoot", "Checks the Secure Boot state measured in the UEFI event log (enabled or disabled) against the expected value", logValue("secureboot", func(s uefi.State) string { return s.SecureBoot }), true),
		rule("uefi_logAuthorities", "Checks that every certificate that verified a boot image, as measured in the UEFI event log, is one of the expected value's authorities", logAuthorities, true),
		rule("uefi_logDMAProtection", "Checks whether the firmware reported DMA protection disabled in the UEFI event log, against the expected value", logValue("dmaprotection", func(s uefi.State) string { return s.DMAProtection }), true),
		rule("uefi_logFirmwareVersion", "Checks the firmware (S-CRTM) version measured in the UEFI event log against the expected value", logValue("firmwareversion", func(s uefi.State) string { return s.FirmwareVersion }), true),
		rule("uefi_logMatchesEfivars", "On a uefi/efivars claim: checks that SecureBoot, PK, KEK, db and dbx as the OS reports them match what was measured in the UEFI event log (parameter uefilogcid)", logMatchesEfivars, false),
	}
	for _, v := range uefi.StateVariables {
		key := v.Key
		rules = append(rules, rule("uefi_log"+v.Name, "Checks the "+v.Name+" variable measured in the UEFI event log against the expected value",
			logValue(key, func(s uefi.State) string { return s.Variables[key] }), true))
	}
	return rules
}

// measured reads the measured state from a uefi/eventlog claim; msg is set
// if it can't.
func measured(claim structures.Claim) (uefi.State, string) {
	if f := claim.Header.Intent.Function; f != "uefi/eventlog" {
		return uefi.State{}, fmt.Sprintf("This rule runs on a uefi/eventlog claim, not %s", f)
	}
	b64, _ := claim.Body["eventlog"].(string)
	el, err := uefi.DecodeEventLog(b64)
	if err != nil {
		return uefi.State{}, "The event log could not be parsed: " + err.Error()
	}
	st, err := uefi.MeasuredState(el)
	if err != nil {
		return uefi.State{}, err.Error()
	}
	return st, ""
}

// logValue checks one measured value against the expected value's entry key.
func logValue(key string, get func(uefi.State) string) structures.RuleCall {
	return func(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
		want, ok := ev.EVS[key]
		if !ok || want == nil {
			return structures.Fail, fmt.Sprintf("Expected value %v does not mention %v", ev.Name, key), nil
		}
		st, msg := measured(claim)
		if msg != "" {
			return structures.Fail, msg, nil
		}
		got := get(st)
		if strings.EqualFold(got, fmt.Sprintf("%v", want)) {
			return structures.Success, fmt.Sprintf("%s is %s, as expected", key, got), nil
		}
		return structures.Fail, fmt.Sprintf("%s is %s, expected %v", key, got, want), nil
	}
}

func logAuthorities(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	raw, ok := ev.EVS["authorities"]
	if !ok || raw == nil {
		return structures.Fail, fmt.Sprintf("Expected value %v does not mention authorities", ev.Name), nil
	}
	// a list; from MongoDB it may be a primitive.A rather than []interface{}
	allowed := map[string]bool{}
	if l := reflect.ValueOf(raw); l.Kind() == reflect.Slice {
		for i := 0; i < l.Len(); i++ {
			allowed[strings.ToLower(fmt.Sprintf("%v", l.Index(i).Interface()))] = true
		}
	} else {
		return structures.Fail, fmt.Sprintf("Expected value %v: authorities is not a list", ev.Name), nil
	}
	st, msg := measured(claim)
	if msg != "" {
		return structures.Fail, msg, nil
	}
	var unknown []string
	for _, a := range st.Authorities {
		if !allowed[a] {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return structures.Fail, fmt.Sprintf("%d of %d authorities are not expected: %s", len(unknown), len(st.Authorities), strings.Join(unknown, ", ")), nil
	}
	return structures.Success, fmt.Sprintf("all %d authorities are expected", len(st.Authorities)), nil
}

// logMatchesEfivars runs on a uefi/efivars claim and compares what the OS
// reports with the Secure Boot variables measured in the uefi/eventlog claim
// given by the parameter uefilogcid (as tpm2_uefiLogReplay takes it). A
// difference means the OS reports something else than was measured at boot,
// or the variables changed since (e.g. a dbx update waiting for a reboot).
func logMatchesEfivars(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
	if f := claim.Header.Intent.Function; f != "uefi/efivars" {
		return structures.Fail, fmt.Sprintf("This rule runs on a uefi/efivars claim, not %s", f), nil
	}
	body, err := utilities.ParseEfiVars(claim.Body)
	if err != nil {
		return structures.Fail, "Parsing UEFI variables failed", err
	}
	cid, _ := parameter["uefilogcid"].(string)
	if cid == "" {
		return structures.Fail, "Rule parameter uefilogcid (a uefi/eventlog claim itemid) is missing", nil
	}
	logc, err := getClaim(cid)
	if err != nil {
		return structures.Fail, fmt.Sprintf("Claim %s given in uefilogcid could not be read: %v", cid, err), nil
	}
	if logc.BodyType == structures.CLAIMERROR {
		return structures.Fail, fmt.Sprintf("Claim %s given in uefilogcid is an error claim", cid), nil
	}
	if logc.Header.Element.ItemID != claim.Header.Element.ItemID {
		return structures.Fail, fmt.Sprintf("Claim %s given in uefilogcid is for another element", cid), nil
	}
	st, msg := measured(logc)
	if msg != "" {
		return structures.Fail, msg, nil
	}

	var diffs []string
	compare := func(name string) {
		v := variableNamed(name)
		m, measured := st.Data[name]
		r, reported := body.Find(name, v.GUID)
		switch {
		case !measured && !reported:
		case measured && !reported:
			diffs = append(diffs, name+" measured but not reported")
		case !measured && reported:
			diffs = append(diffs, name+" reported but not measured")
		case r.Lossy || r.Error != "":
			diffs = append(diffs, name+" could not be compared (the agent's copy is incomplete)")
		case !bytes.Equal(m, r.Data):
			diffs = append(diffs, fmt.Sprintf("%s differs (measured %s, reported %s)", name, short(m), short(r.Data)))
		}
	}
	compare("SecureBoot")
	for _, v := range uefi.StateVariables {
		compare(v.Name)
	}
	if len(diffs) > 0 {
		return structures.Fail, "Measured and reported differ: " + strings.Join(diffs, "; "), nil
	}
	return structures.Success, "SecureBoot, PK, KEK, db and dbx as reported match what was measured at boot", nil
}

func variableNamed(name string) uefi.Variable {
	for _, v := range Variables {
		if v.Name == name {
			return v
		}
	}
	return uefi.Variable{Name: name}
}

func short(b []byte) string {
	if len(b) <= 8 {
		return hex.EncodeToString(b)
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:6]) + "…"
}
