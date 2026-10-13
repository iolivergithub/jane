package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Vendor GUIDs of the UEFI variables checked by janeserver's UEFI_ rules.
const (
	efiGlobalVariableGUID        = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	efiImageSecurityDatabaseGUID = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	shimLockGUID                 = "605dab50-e046-4300-abb6-3dd810dd8b23"
)

type uefiVariable struct{ Name, GUID string }

// uefiRuleVariables are the variables janeserver's UEFI_<name> rules check
// (janeserver/rules/uefirules, Variables). Keep the two lists in step: a
// variable missing here has no entry in the expected value, so its rule fails.
var uefiRuleVariables = []uefiVariable{
	{"SecureBoot", efiGlobalVariableGUID},
	{"SetupMode", efiGlobalVariableGUID},
	{"AuditMode", efiGlobalVariableGUID},
	{"DeployedMode", efiGlobalVariableGUID},
	{"VendorKeys", efiGlobalVariableGUID},
	{"PK", efiGlobalVariableGUID},
	{"KEK", efiGlobalVariableGUID},
	{"db", efiImageSecurityDatabaseGUID},
	{"MokSBStateRT", shimLockGUID},
	{"dbx", efiImageSecurityDatabaseGUID},
	{"SbatLevelRT", shimLockGUID},
	{"MokListRT", shimLockGUID},
	{"MokListXRT", shimLockGUID},
	{"PKDefault", efiGlobalVariableGUID},
	{"KEKDefault", efiGlobalVariableGUID},
	{"dbDefault", efiGlobalVariableGUID},
	{"dbxDefault", efiGlobalVariableGUID},
}

// uefiInlineLimit is the longest value recorded as hex; longer values are
// recorded as sha256:<hex>. Same cutoff as janeserver's claim view.
const uefiInlineLimit = 32

// uefiAbsent is the expected value meaning "this variable must not exist".
const uefiAbsent = "absent"

type efivarsBody struct {
	Efivars *[]efivarEntry `json:"efivars"`
}

type efivarEntry struct {
	Name       string  `json:"name"`
	GUID       string  `json:"guid"`
	ErrorValue string  `json:"errorvalue"`
	RawValue   *string `json:"rawvalue"`
}

// uefiEVs builds the evs map for the UEFI_ rules from the body of a
// uefi/efivars claim: one entry per rule variable, as hex, sha256:<hex> or
// "absent".
//
// tarzan sends every variable efivarfs lists, so a variable missing from the
// claim is absent on the machine. tarzan does not report a failure to list
// variables, though, so a claim with no variables at all is refused rather
// than recorded as "everything absent". A variable the agent could not read
// is left out with a warning (its rule will then fail), and a claim from an
// agent that does not send rawvalue is refused, since its values are lossy.
func uefiEVs(body json.RawMessage, out io.Writer) (map[string]any, error) {
	var b efivarsBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	if b.Efivars == nil {
		return nil, errors.New("claim body has no efivars")
	}
	if len(*b.Efivars) == 0 {
		return nil, errors.New("claim lists no UEFI variables; the agent may not have been able to read efivarfs, so no expected value was recorded")
	}

	type key struct{ name, guid string }
	byKey := map[key]efivarEntry{}
	for _, v := range *b.Efivars {
		byKey[key{v.Name, strings.ToLower(v.GUID)}] = v
	}

	evs := map[string]any{}
	for _, rv := range uefiRuleVariables {
		v, ok := byKey[key{rv.Name, rv.GUID}]
		if !ok {
			evs[rv.Name] = uefiAbsent
			continue
		}
		if v.ErrorValue != "" {
			fmt.Fprintf(out, "  ... warning: %s could not be read by the agent (%s); left out of the expected value, so UEFI_%s will fail\n", rv.Name, v.ErrorValue, rv.Name)
			continue
		}
		if v.RawValue == nil {
			return nil, fmt.Errorf("%s has no rawvalue; the agent is too old to send UEFI variables exactly, update tarzan", rv.Name)
		}
		data, err := base64.StdEncoding.DecodeString(*v.RawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: rawvalue is not base64: %w", rv.Name, err)
		}
		evs[rv.Name] = uefiValue(data)
	}
	return evs, nil
}

// uefiValue renders a variable's bytes the way the UEFI_ rules compare them.
func uefiValue(data []byte) string {
	if len(data) <= uefiInlineLimit {
		return hex.EncodeToString(data)
	}
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
