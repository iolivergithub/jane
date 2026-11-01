package utilities

// The measured UEFI state in a UEFI event log (#134): what the firmware
// measured about Secure Boot into PCR 7 (and its version into PCR 0), as
// opposed to the variables the running OS reports (uefi/efivars).
//
// jp computes the same values for its expected-value type uefieventlog
// (provisioner/uefieventlog.go); keep the two in step. Both are tested
// against testdata/cos-85-uefistate.json.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/google/go-eventlog/extract"
	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// UEFIStateVariables are the Secure Boot variables whose measured data the
// uefi_log rules check, by name in the expected value.
var UEFIStateVariables = []struct{ Name, Key string }{
	{"PK", "pk"}, {"KEK", "kek"}, {"db", "db"}, {"dbx", "dbx"},
}

// UEFIState is the measured state, in the form of an expected value.
type UEFIState struct {
	Bank            string            // the digest bank the events were read with
	SecureBoot      string            // "enabled" or "disabled"
	Variables       map[string]string // "pk", "kek", "db", "dbx" -> "sha256:<hex>" of the measured data, or "absent"
	Data            map[string][]byte // variable name (SecureBoot, PK, KEK, db, dbx) -> the measured data
	Authorities     []string          // "sha256:<hex>" of each certificate that verified an image, sorted, once each
	DMAProtection   string            // "disabled" if the firmware said so, else "notreported"
	FirmwareVersion string            // the S-CRTM version (PCR 0), "" if none
}

// MeasuredUEFIState reads the measured UEFI state from an event log. The
// events are not replayed here: they are as good as the log, which
// tpm2_uefiLogReplay checks against the PCRs. A log go-eventlog considers
// unsafe (a UEFI debugger, duplicate variables, digests that don't match
// the data, ...) is an error.
func MeasuredUEFIState(el *tcg.EventLog) (UEFIState, error) {
	var st UEFIState
	if len(el.Algs) == 0 {
		return st, fmt.Errorf("the event log has no digest banks")
	}
	alg := el.Algs[0]
	for _, a := range el.Algs {
		if a == register.HashSHA256 {
			alg = a
		}
	}
	st.Bank = strings.ToLower(alg.String())
	events := el.Events(alg)

	sb, err := extract.ParseSecurebootState(events, extract.TPMRegisterConfig)
	if err != nil {
		return st, fmt.Errorf("the event log's Secure Boot state could not be read: %w", err)
	}
	st.SecureBoot = "disabled"
	if sb.Enabled {
		st.SecureBoot = "enabled"
	}
	st.DMAProtection = "notreported"
	if sb.DMAProtectionDisabled {
		st.DMAProtection = "disabled"
	}

	seen := map[string]bool{}
	for _, c := range append(sb.PreSeparatorAuthority, sb.PostSeparatorAuthority...) {
		h := sha256.Sum256(c.Raw)
		fp := "sha256:" + hex.EncodeToString(h[:])
		if !seen[fp] {
			seen[fp] = true
			st.Authorities = append(st.Authorities, fp)
		}
	}
	sort.Strings(st.Authorities)

	st.Data = map[string][]byte{}
	for _, e := range events {
		switch {
		case e.Index == 7 && e.Type == tcg.EFIVariableDriverConfig:
			v, err := tcg.ParseUEFIVariableData(bytes.NewReader(e.Data))
			if err == nil {
				st.Data[v.VarName()] = v.VariableData
			}
		case e.Index == 0 && e.Type == tcg.SCRTMVersion && st.FirmwareVersion == "":
			st.FirmwareVersion = SCRTMVersion(e.Data)
		}
	}
	st.Variables = map[string]string{}
	for _, v := range UEFIStateVariables {
		d, ok := st.Data[v.Name]
		if !ok {
			st.Variables[v.Key] = "absent"
			continue
		}
		h := sha256.Sum256(d)
		st.Variables[v.Key] = "sha256:" + hex.EncodeToString(h[:])
	}
	return st, nil
}

// EVS is the state as the evs of an expected value (jp's uefieventlog type).
func (st UEFIState) EVS() map[string]interface{} {
	evs := map[string]interface{}{
		"secureboot":      st.SecureBoot,
		"dmaprotection":   st.DMAProtection,
		"firmwareversion": st.FirmwareVersion,
	}
	auth := make([]interface{}, len(st.Authorities))
	for i, a := range st.Authorities {
		auth[i] = a
	}
	evs["authorities"] = auth
	for k, v := range st.Variables {
		evs[k] = v
	}
	return evs
}

// SCRTMVersion decodes an EV_S_CRTM_VERSION event: usually a NUL-terminated
// UCS-2 string, sometimes plain bytes; anything else is shown as hex.
func SCRTMVersion(b []byte) string {
	if len(b) >= 2 && len(b)%2 == 0 {
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		}
		s := strings.TrimRight(string(utf16.Decode(u)), "\x00")
		if printable(s) {
			return s
		}
	}
	if s := strings.TrimRight(string(b), "\x00"); printable(s) {
		return s
	}
	return "hex:" + hex.EncodeToString(b)
}

func printable(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}
