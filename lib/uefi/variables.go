// Package uefi holds what janeserver and jp must agree on about UEFI: the
// variables the UEFI_<name> rules check and how their values are written in
// an expected value, and the measured UEFI state in an event log that the
// uefi_log rules check (#134). jp records expected values with it and
// janeserver checks claims with it, so the two cannot drift apart.
package uefi

import (
	"crypto/sha256"
	"encoding/hex"
)

// Vendor GUIDs of the UEFI variables Jane knows about, in lower case.
const (
	EfiGlobalVariableGUID        = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	EfiImageSecurityDatabaseGUID = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	ShimLockGUID                 = "605dab50-e046-4300-abb6-3dd810dd8b23"
)

// Variable is a UEFI variable that has a UEFI_ rule.
type Variable struct {
	Name        string
	GUID        string
	Description string
}

// RuleVariables lists the variables checked by janeserver's UEFI_ rules, in
// registration order. jp records an expected value entry for each, so a
// variable added here is both checked and recorded.
var RuleVariables = []Variable{
	// static, security relevant
	{"SecureBoot", EfiGlobalVariableGUID, "Secure Boot enabled state"},
	{"SetupMode", EfiGlobalVariableGUID, "Setup mode; 1 means no PK is enrolled and keys can be enrolled without authentication"},
	{"AuditMode", EfiGlobalVariableGUID, "Audit mode"},
	{"DeployedMode", EfiGlobalVariableGUID, "Deployed mode"},
	{"VendorKeys", EfiGlobalVariableGUID, "Whether the Secure Boot keys are unmodified from the vendor's"},
	{"PK", EfiGlobalVariableGUID, "Platform Key"},
	{"KEK", EfiGlobalVariableGUID, "Key Exchange Keys"},
	{"db", EfiImageSecurityDatabaseGUID, "Allowed signature database"},
	{"MokSBStateRT", ShimLockGUID, "Shim validation state; normally absent, present means shim signature checking is disabled"},
	// expected to be updated deliberately
	{"dbx", EfiImageSecurityDatabaseGUID, "Forbidden signature database (revocations)"},
	{"SbatLevelRT", ShimLockGUID, "SBAT revocation level"},
	{"MokListRT", ShimLockGUID, "Machine Owner Keys"},
	{"MokListXRT", ShimLockGUID, "Forbidden Machine Owner Keys"},
	{"PKDefault", EfiGlobalVariableGUID, "Factory default Platform Key"},
	{"KEKDefault", EfiGlobalVariableGUID, "Factory default Key Exchange Keys"},
	{"dbDefault", EfiGlobalVariableGUID, "Factory default allowed signature database"},
	{"dbxDefault", EfiGlobalVariableGUID, "Factory default forbidden signature database"},
}

// Absent is the expected value meaning "this variable must not exist".
const Absent = "absent"

// InlineLimit is the longest value written as hex; longer values are written
// as sha256:<hex>.
const InlineLimit = 32

// Value writes a variable's bytes as the UEFI_ rules compare them: hex up to
// InlineLimit bytes, sha256:<hex> of the bytes beyond that.
func Value(data []byte) string {
	if len(data) <= InlineLimit {
		return hex.EncodeToString(data)
	}
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
