package main

// Expected-value type uefieventlog (#134): the measured UEFI state in a
// uefi/eventlog claim, for janeserver's uefi_log rules. The same values as
// janeserver's utilities.MeasuredUEFIState; keep the two in step (both are
// tested against janeserver/utilities/testdata/cos-85-uefistate.json).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/google/go-eventlog/extract"
	"github.com/google/go-eventlog/register"
	"github.com/google/go-eventlog/tcg"
)

// uefiEventLogEVs returns the expected value for a uefi/eventlog claim's
// body. An error means the log can't be read; jp then records nothing.
func uefiEventLogEVs(body json.RawMessage) (map[string]any, error) {
	var b struct {
		Eventlog string `json:"eventlog"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(b.Eventlog)
	if err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("the claim has no event log")
	}
	el, err := tcg.ParseEventLog(raw, tcg.ParseOpts{AllowPadding: true})
	if err != nil {
		return nil, fmt.Errorf("parsing the event log: %w", err)
	}
	if len(el.Algs) == 0 {
		return nil, fmt.Errorf("the event log has no digest banks")
	}
	alg := el.Algs[0]
	for _, a := range el.Algs {
		if a == register.HashSHA256 {
			alg = a
		}
	}
	events := el.Events(alg)
	sb, err := extract.ParseSecurebootState(events, extract.TPMRegisterConfig)
	if err != nil {
		return nil, fmt.Errorf("the event log's Secure Boot state could not be read: %w", err)
	}

	evs := map[string]any{"secureboot": "disabled", "dmaprotection": "notreported", "firmwareversion": ""}
	if sb.Enabled {
		evs["secureboot"] = "enabled"
	}
	if sb.DMAProtectionDisabled {
		evs["dmaprotection"] = "disabled"
	}
	seen := map[string]bool{}
	auth := []string{}
	for _, c := range append(sb.PreSeparatorAuthority, sb.PostSeparatorAuthority...) {
		h := sha256.Sum256(c.Raw)
		fp := "sha256:" + hex.EncodeToString(h[:])
		if !seen[fp] {
			seen[fp] = true
			auth = append(auth, fp)
		}
	}
	sort.Strings(auth)
	evs["authorities"] = auth

	data := map[string][]byte{}
	for _, e := range events {
		switch {
		case e.Index == 7 && e.Type == tcg.EFIVariableDriverConfig:
			if v, err := tcg.ParseUEFIVariableData(bytes.NewReader(e.Data)); err == nil {
				data[v.VarName()] = v.VariableData
			}
		case e.Index == 0 && e.Type == tcg.SCRTMVersion && evs["firmwareversion"] == "":
			evs["firmwareversion"] = scrtmVersion(e.Data)
		}
	}
	for name, key := range map[string]string{"PK": "pk", "KEK": "kek", "db": "db", "dbx": "dbx"} {
		d, ok := data[name]
		if !ok {
			evs[key] = "absent"
			continue
		}
		h := sha256.Sum256(d)
		evs[key] = "sha256:" + hex.EncodeToString(h[:])
	}
	return evs, nil
}

// scrtmVersion decodes an EV_S_CRTM_VERSION event: usually a NUL-terminated
// UCS-2 string, sometimes plain bytes; anything else is shown as hex.
func scrtmVersion(b []byte) string {
	if len(b) >= 2 && len(b)%2 == 0 {
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		}
		if s := strings.TrimRight(string(utf16.Decode(u)), "\x00"); printableASCII(s) {
			return s
		}
	}
	if s := strings.TrimRight(string(b), "\x00"); printableASCII(s) {
		return s
	}
	return "hex:" + hex.EncodeToString(b)
}

func printableASCII(s string) bool {
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
