package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-eventlog/tcg"
	"github.com/iolivergithub/jane/lib/uefi"
)

// A claim body is compared as it is ("body") and, for the kinds of claim
// Jane's rules understand, also decoded into the values those rules and
// their expected values work with ("decoded"): what is measured in a UEFI
// event log, each UEFI variable as the UEFI_ rules write it, the parts of a
// TPM quote that are not new on every call, the files in an IMA log. Small
// differences in a large blob then read as what they mean.

// decodedView returns the decoded view of a claim body, and a title for it,
// or nil if the claim's function has none.
func decodedView(function string, body map[string]any) (map[string]any, string) {
	if body == nil {
		return nil, ""
	}
	switch function {
	case "tpm2/quote":
		if q, ok := body["quote"].(map[string]any); ok {
			return attestView(q, "attested"), "TPM quote: what it attests (without the nonce, clock and signature)"
		}
	case "tpm2/gettime":
		if q, ok := body["time"].(map[string]any); ok {
			return attestView(q, "time"), "TPM time: counters and firmware (without the nonce, clock and signature)"
		}
	case "uefi/eventlog":
		if s, ok := body["eventlog"].(string); ok {
			return eventLogView(s), "UEFI event log, decoded: measured Secure Boot state and events"
		}
	case "uefi/efivars":
		if vs, ok := body["efivars"].([]any); ok {
			return efivarsView(vs), "UEFI variables as the UEFI_ rules compare them (hex, or sha256: of values over 32 bytes)"
		}
	case "ima/asciilog":
		if s, ok := body["asciilog"].(string); ok {
			return imaView(s), "IMA log, decoded: boot aggregate and the files measured"
		}
	}
	return nil, ""
}

// volatilePrefixes are the parts of a claim body that differ on every call
// by design (nonce, clock, signature): reported, but marked, so that they
// can be hidden and do not make every claim look changed.
var volatilePrefixes = map[string][]string{
	"tpm2/quote":   {"quote.extradata", "quote.clockinfo.clock", "signature", "attest", "rawsignature"},
	"tpm2/gettime": {"time.extradata", "time.clockinfo.clock", "time.time", "signature", "attest", "rawsignature"},
}

func isVolatile(function, path string) bool {
	for _, p := range volatilePrefixes[function] {
		if path == p || strings.HasPrefix(path, p+".") || strings.HasPrefix(path, p+"[") {
			return true
		}
	}
	return false
}

// attestView keeps the fields of a TPM attest structure that stay the same
// from call to call unless something happened: the attested PCR selection
// and digest, the firmware version, the reset and restart counts, the safe
// flag and the signer.
func attestView(q map[string]any, attested string) map[string]any {
	v := map[string]any{}
	for _, k := range []string{"magic", "type", "qualifiedsigner", "firmwareVersion"} {
		if x, ok := q[k]; ok {
			v[k] = x
		}
	}
	if ci, ok := q["clockinfo"].(map[string]any); ok {
		for _, k := range []string{"resetcount", "restartcount", "safe"} {
			if x, ok := ci[k]; ok {
				v[k] = x
			}
		}
	}
	if a, ok := q[attested].(map[string]any); ok && attested == "attested" {
		for _, k := range []string{"pcrselect", "pcrdigest"} {
			if x, ok := a[k]; ok {
				v[k] = x
			}
		}
	}
	return v
}

// efivarsView is each variable as the UEFI_ rules compare it, by name (with
// the GUID where a name is used twice).
func efivarsView(vs []any) map[string]any {
	names := map[string]int{}
	for _, x := range vs {
		if m, ok := x.(map[string]any); ok {
			n, _ := m["name"].(string)
			names[n]++
		}
	}
	out := map[string]any{}
	for _, x := range vs {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		guid, _ := m["guid"].(string)
		key := name
		if names[name] > 1 {
			key = name + " (" + guid + ")"
		}
		if e, _ := m["errorvalue"].(string); e != "" {
			out[key] = "error: " + e
			continue
		}
		if raw, ok := m["rawvalue"].(string); ok {
			if b, err := base64.StdEncoding.DecodeString(raw); err == nil {
				out[key] = uefi.Value(b)
				continue
			}
		}
		if s, ok := m["value"].(string); ok {
			out[key] = uefi.Value([]byte(s)) + " (from text, may be lossy)"
		}
	}
	return map[string]any{"variables": out}
}

// eventLogView is the measured state as the uefi_log rules read it, and
// every event by PCR and position, so that a changed event can be named.
func eventLogView(b64 string) map[string]any {
	el, err := uefi.DecodeEventLog(b64)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	v := map[string]any{}
	st, err := uefi.MeasuredState(el)
	if err != nil {
		v["stateerror"] = err.Error()
	} else {
		v["secureboot"] = st.SecureBoot
		v["dmaprotection"] = st.DMAProtection
		v["firmwareversion"] = st.FirmwareVersion
		v["bank"] = st.Bank
		for k, x := range st.Variables {
			v[k] = x
		}
		auth := map[string]any{}
		for _, a := range st.Authorities {
			auth[a] = "present"
		}
		v["authorities"] = auth
	}
	if len(el.Algs) > 0 {
		alg := el.Algs[0]
		for _, a := range el.Algs {
			if strings.EqualFold(a.String(), "SHA256") {
				alg = a
			}
		}
		events := map[string]any{}
		count := map[string]any{}
		seq := map[int]int{}
		for _, e := range el.Events(alg) {
			seq[e.Index]++
			key := fmt.Sprintf("PCR%02d #%03d", e.Index, seq[e.Index])
			events[key] = strings.TrimSuffix(fmt.Sprintf("%s | %s | %s", e.Type.String(), hex.EncodeToString(e.Digest), eventNote(e)), " | ")
		}
		for pcr, n := range seq {
			count[fmt.Sprintf("PCR%02d", pcr)] = n
		}
		v["events"] = events
		v["eventsperpcr"] = count
	}
	return v
}

// eventNote names what an event is about, where that is easy to tell.
func eventNote(e tcg.Event) string {
	switch e.Type {
	case tcg.EFIVariableDriverConfig, tcg.EFIVariableBoot, tcg.EFIVariableBoot2, tcg.EFIVariableAuthority:
		if d, err := tcg.ParseUEFIVariableData(bytes.NewReader(e.Data)); err == nil {
			return d.VarName()
		}
	case tcg.SCRTMVersion:
		return uefi.SCRTMVersion(e.Data)
	case tcg.Separator, tcg.EFIAction, tcg.Action, tcg.Ipl, tcg.EventTag:
		return printableText(e.Data)
	}
	return ""
}

func printableText(b []byte) string {
	s := strings.TrimRight(string(b), "\x00")
	if s == "" || len(s) > 240 {
		return ""
	}
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return ""
		}
	}
	return s
}

// imaView reads an IMA ASCII log (base64, as tarzan sends it): the number of
// entries, the boot aggregate, and each file with its latest hash.
func imaView(b64 string) map[string]any {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return map[string]any{"error": "base64: " + err.Error()}
	}
	files := map[string]any{}
	entries := 0
	boot := ""
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 5 {
			continue
		}
		entries++
		// PCR, template hash, template, file hash, path (ima, ima-ng, ima-sig)
		path, hash := f[4], f[3]
		if path == "boot_aggregate" {
			boot = hash
			continue
		}
		files[path] = hash
	}
	return map[string]any{"entries": entries, "bootaggregate": boot, "files": files}
}

// ---------------------------------------------------------------- names

// pcrNames is the register naming schemes from Jane (GET /pcrnames): scheme
// -> register -> short name.
type pcrNames struct {
	def     string
	schemes map[string]map[string]string
}

func (p *pcrNames) name(scheme, register string) string {
	if p == nil {
		return ""
	}
	if scheme == "" {
		scheme = p.def
	}
	r := strings.TrimLeft(register, "0")
	if r == "" {
		r = "0"
	}
	return p.schemes[scheme][r]
}

// evKeyNames describes the keys of the expected values jp records, by the
// rules that read them.
func evKeyName(key string) string {
	switch key {
	case "attestedValue":
		return "attested PCR digest (tpm2_attestedValue)"
	case "firmwareVersion":
		return "TPM firmware version (tpm2_firmware)"
	case "machineid":
		return "machine ID (sys_machineID_Agent)"
	case "issuerurl":
		return "EK certificate issuer URL (tpm2_ekCertIssuer)"
	case "secureboot":
		return "measured Secure Boot state (uefi_logSecureBoot)"
	case "pk", "kek", "db", "dbx":
		return "measured " + strings.ToUpper(key) + " (uefi_log" + map[string]string{"pk": "PK", "kek": "KEK", "db": "db", "dbx": "dbx"}[key] + ")"
	case "authorities":
		return "measured boot authorities (uefi_logAuthorities)"
	case "dmaprotection":
		return "measured DMA protection (uefi_logDMAProtection)"
	case "firmwareversion":
		return "measured firmware version (uefi_logFirmwareVersion)"
	}
	for _, v := range uefi.RuleVariables {
		if v.Name == key {
			return "UEFI variable " + v.Name + ", " + v.Description + " (UEFI_" + v.Name + ")"
		}
	}
	return ""
}

func uefiVariableDescription(name string) string {
	for _, v := range uefi.RuleVariables {
		if v.Name == name {
			return v.Description
		}
	}
	return ""
}

func atoi(s string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 0, 64)
	return n, err == nil
}

func sortedStrings(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
