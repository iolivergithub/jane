// Package uefirules checks UEFI variables in a uefi/efivars claim against an expected value.
//
// There is one rule per variable, named UEFI_<variable>. All of them read the same
// expected value, whose evs map holds one entry per variable, keyed by the variable name:
//
//	"SecureBoot": "01"                 the value as hex
//	"PK":         "sha256:3c4f...e1"   the SHA-256 of the value, for long values
//	"MokSBStateRT": "absent"           the variable must not be present
//
// A variable not mentioned in the expected value fails, as the tpm2_ rules do when their
// entry is missing (MissingExpectedValue is only for when there is no expected value at
// all). A variable that is mentioned passes if the claim matches and fails otherwise.
package uefirules

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"a10/structures"
	"a10/utilities"
)

// Variable is a UEFI variable that has a UEFI_ rule.
type Variable struct {
	Name        string
	GUID        string
	Description string
}

// Variables lists the variables checked by the UEFI_ rules, in registration order.
var Variables = []Variable{
	// static, security relevant
	{"SecureBoot", utilities.EfiGlobalVariableGUID, "Secure Boot enabled state"},
	{"SetupMode", utilities.EfiGlobalVariableGUID, "Setup mode; 1 means no PK is enrolled and keys can be enrolled without authentication"},
	{"AuditMode", utilities.EfiGlobalVariableGUID, "Audit mode"},
	{"DeployedMode", utilities.EfiGlobalVariableGUID, "Deployed mode"},
	{"VendorKeys", utilities.EfiGlobalVariableGUID, "Whether the Secure Boot keys are unmodified from the vendor's"},
	{"PK", utilities.EfiGlobalVariableGUID, "Platform Key"},
	{"KEK", utilities.EfiGlobalVariableGUID, "Key Exchange Keys"},
	{"db", utilities.EfiImageSecurityDatabaseGUID, "Allowed signature database"},
	{"MokSBStateRT", utilities.ShimLockGUID, "Shim validation state; normally absent, present means shim signature checking is disabled"},
	// expected to be updated deliberately
	{"dbx", utilities.EfiImageSecurityDatabaseGUID, "Forbidden signature database (revocations)"},
	{"SbatLevelRT", utilities.ShimLockGUID, "SBAT revocation level"},
	{"MokListRT", utilities.ShimLockGUID, "Machine Owner Keys"},
	{"MokListXRT", utilities.ShimLockGUID, "Forbidden Machine Owner Keys"},
	{"PKDefault", utilities.EfiGlobalVariableGUID, "Factory default Platform Key"},
	{"KEKDefault", utilities.EfiGlobalVariableGUID, "Factory default Key Exchange Keys"},
	{"dbDefault", utilities.EfiGlobalVariableGUID, "Factory default allowed signature database"},
	{"dbxDefault", utilities.EfiGlobalVariableGUID, "Factory default forbidden signature database"},
}

// RuleName is the name of the rule that checks the named variable.
func RuleName(variable string) string {
	return "UEFI_" + variable
}

func Registration() []structures.Rule {
	rules := make([]structures.Rule, 0, len(Variables))
	for _, v := range Variables {
		rules = append(rules, structures.Rule{
			Name:         RuleName(v.Name),
			Description:  fmt.Sprintf("Checks the UEFI variable %v (%v) against the expected value", v.Name, v.Description),
			CallFunction: ruleFor(v),
			NeedsEV:      true,
		})
	}
	return rules
}

func ruleFor(v Variable) structures.RuleCall {
	return func(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {
		return checkVariable(v, claim, ev)
	}
}

const absent = "absent"

func checkVariable(v Variable, claim structures.Claim, ev structures.ExpectedValue) (structures.ResultValue, string, error) {
	evEntry, ok := ev.EVS[v.Name]
	if !ok || evEntry == nil {
		return structures.Fail, fmt.Sprintf("Expected value %v does not mention %v", ev.Name, v.Name), nil
	}
	expected := normalise(fmt.Sprintf("%v", evEntry))

	body, err := utilities.ParseEfiVars(claim.Body)
	if err != nil {
		return structures.Fail, "Parsing UEFI variables failed", err
	}

	got, present := body.Find(v.Name, v.GUID)

	if expected == absent {
		if present {
			return structures.Fail, fmt.Sprintf("%v is present but expected to be absent", v.Name), nil
		}
		return structures.Success, fmt.Sprintf("%v is absent as expected", v.Name), nil
	}

	if !present {
		return structures.Fail, fmt.Sprintf("%v is not present but expected %v", v.Name, expected), nil
	}
	if got.Error != "" {
		return structures.Fail, fmt.Sprintf("%v could not be read by the agent: %v", v.Name, got.Error), nil
	}
	if got.Lossy {
		return structures.VerifyCallFailure, fmt.Sprintf("%v was sent as text by an older agent and bytes were lost in transit, so it cannot be compared; update tarzan", v.Name), nil
	}

	if strings.HasPrefix(expected, "sha256:") {
		h := sha256.Sum256(got.Data)
		claimed := "sha256:" + hex.EncodeToString(h[:])
		if claimed == expected {
			return structures.Success, fmt.Sprintf("%v matches (%v)", v.Name, claimed), nil
		}
		return structures.Fail, fmt.Sprintf("Got %v for %v but expected %v", claimed, v.Name, expected), nil
	}

	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return structures.Fail, fmt.Sprintf("Expected value for %v is neither hex, sha256:<hex> nor %q: %v", v.Name, absent, evEntry), err
	}
	if bytes.Equal(got.Data, expectedBytes) {
		return structures.Success, fmt.Sprintf("%v matches (%v)", v.Name, describe(got.Data)), nil
	}
	return structures.Fail, fmt.Sprintf("Got %v for %v but expected %v", describe(got.Data), v.Name, expected), nil
}

// normalise lower-cases an expected value and removes whitespace, colons and a 0x prefix
// from hex, so values copied from hex dumps or other tools still compare.
func normalise(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	prefix := ""
	if strings.HasPrefix(s, "sha256:") {
		prefix, s = "sha256:", s[len("sha256:"):]
	}
	s = strings.TrimPrefix(s, "0x")
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ':' {
			return -1
		}
		return r
	}, s)
	return prefix + s
}

// describe shows short values as hex and long ones by their SHA-256, to keep messages readable.
func describe(b []byte) string {
	if len(b) <= 32 {
		return hex.EncodeToString(b)
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
