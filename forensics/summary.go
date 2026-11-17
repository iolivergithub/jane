package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SummaryLine is one line of the readable account of a change set. Paths
// are the differences it speaks for, so that the page can drop the line when
// all of them are ignored or volatile.
type SummaryLine struct {
	Text  string   `json:"t"`
	Paths []string `json:"p,omitempty"`
	V     bool     `json:"v,omitempty"` // only volatile differences
}

// sumCtx is what a summary needs to know besides the differences.
type sumCtx struct {
	function  string // the claim's intent function, e.g. tpm2/pcrs
	pcrScheme string // the element's register naming scheme (tpm2.pcrnaming)
	names     *pcrNames
	decoded   bool // the claim also has a decoded change set
}

// genericLimit is how many differences are spelt out one by one.
const genericLimit = 12

// short writes a value for a summary: long values by their ends.
func short(v *Value) string {
	if v == nil {
		return "—"
	}
	if v.Truncated {
		return fmt.Sprintf("%s… (%d bytes, sha256 %s…)", firstN(v.Text, 16), v.Len, firstN(v.SHA256, 12))
	}
	t := v.Text
	if len(t) > 48 {
		return t[:20] + "…" + t[len(t)-12:]
	}
	if t == "" {
		return `""`
	}
	return t
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func pathsOf(ds []Diff) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Path
	}
	return out
}

func allVolatile(ds []Diff) bool {
	for _, d := range ds {
		if !d.V {
			return false
		}
	}
	return len(ds) > 0
}

func line(text string, ds ...Diff) SummaryLine {
	return SummaryLine{Text: text, Paths: pathsOf(ds), V: allVolatile(ds)}
}

// describe writes one difference: "label: a → b", "label added: b", ...
func describe(label string, d Diff) string {
	for _, f := range []string{"recordhistory.created", "recordhistory.lastupdated", "recordhistory.archived"} {
		if d.Path == f || strings.HasSuffix(d.Path, "."+f) {
			return fmt.Sprintf("%s: %s → %s", label, nsTime(d.Old), nsTime(d.New))
		}
	}
	switch d.Op {
	case "added":
		return fmt.Sprintf("%s added: %s", label, short(d.New))
	case "removed":
		return fmt.Sprintf("%s removed (was %s)", label, short(d.Old))
	}
	return fmt.Sprintf("%s: %s → %s", label, short(d.Old), short(d.New))
}

// generic spells out up to genericLimit differences and counts the rest,
// volatile ones last and together.
func generic(ds []Diff, label func(string) string) []SummaryLine {
	var out []SummaryLine
	var stable, vol []Diff
	for _, d := range ds {
		if d.V {
			vol = append(vol, d)
		} else {
			stable = append(stable, d)
		}
	}
	for i, d := range stable {
		if i == genericLimit {
			rest := stable[i:]
			out = append(out, line(fmt.Sprintf("… and %d more values changed: %s", len(rest), listPaths(rest, 6)), rest...))
			break
		}
		out = append(out, line(describe(label(d.Path), d), d))
	}
	if len(vol) > 0 {
		out = append(out, line(fmt.Sprintf("%d values that change on every call (nonce, clock, signature): %s", len(vol), listPaths(vol, 4)), vol...))
	}
	return out
}

func listPaths(ds []Diff, n int) string {
	var ps []string
	for i, d := range ds {
		if i == n {
			ps = append(ps, "…")
			break
		}
		ps = append(ps, d.Path)
	}
	return strings.Join(ps, ", ")
}

func plain(p string) string { return p }

// summarize gives the readable account of one change set.
func summarize(what string, ds []Diff, ctx sumCtx) []SummaryLine {
	if len(ds) == 0 {
		return nil
	}
	switch what {
	case "outcome":
		out := []SummaryLine{line("Outcome "+short(ds[0].Old)+" → "+short(ds[0].New), ds[0])}
		for _, d := range ds[1:] {
			if d.Path == "message" && d.New != nil && d.New.Text != "" {
				out = append(out, line("Rule says: "+d.New.Text, d))
			}
		}
		return out
	case "body":
		return summarizeBody(ds, ctx)
	case "decoded":
		return summarizeDecoded(ds, ctx)
	case "element":
		return summarizeElement(ds)
	case "expectedvalue":
		return generic(ds, func(p string) string {
			if k, ok := strings.CutPrefix(p, "recordhistory."); ok && k != "reason" {
				return "Expected value " + k
			}
			if k, ok := strings.CutPrefix(p, "evs."); ok {
				key := k
				if i := strings.IndexAny(k, ".["); i > 0 {
					key = k[:i]
				}
				if n := evKeyName(key); n != "" {
					return "Expected " + n + strings.TrimPrefix(k, key)
				}
				return "Expected value " + k
			}
			return p
		})
	case "intent":
		return generic(ds, func(p string) string { return "Intent " + p })
	case "endpoint":
		return generic(ds, func(p string) string { return "Endpoint " + p })
	}
	return generic(ds, plain)
}

// summarizeBody: PCRs by name; with a decoded change set, the raw body only
// in one line (the decoded lines say what it means).
func summarizeBody(ds []Diff, ctx sumCtx) []SummaryLine {
	if ctx.function == "tpm2/pcrs" {
		return generic(ds, func(p string) string {
			bank, reg, ok := strings.Cut(p, ".")
			if !ok {
				return p
			}
			label := "PCR " + reg
			if n := ctx.names.name(ctx.pcrScheme, reg); n != "" {
				label += " (" + n + ")"
			}
			return label + " in the " + strings.ToUpper(bank) + " bank"
		})
	}
	if ctx.decoded {
		var stable, vol []Diff
		for _, d := range ds {
			if d.V {
				vol = append(vol, d)
			} else {
				stable = append(stable, d)
			}
		}
		var out []SummaryLine
		if len(stable) > 0 {
			out = append(out, line(fmt.Sprintf("Raw claim body: %d %s changed (%s); what they mean is in the decoded lines", len(stable), plural(len(stable), "value"), listPaths(stable, 4)), stable...))
		}
		if len(vol) > 0 {
			out = append(out, line(fmt.Sprintf("%d values that change on every call (nonce, clock, signature)", len(vol)), vol...))
		}
		return out
	}
	if ctx.function == "sys/info" {
		return generic(ds, func(p string) string {
			switch p {
			case "pid":
				return "Trust agent process ID (a new value means the agent was restarted)"
			case "machineid":
				return "Machine ID"
			case "hostname":
				return "Host name"
			case "unsafe":
				return "Trust agent unsafe mode"
			}
			return p
		})
	}
	return generic(ds, plain)
}

func summarizeDecoded(ds []Diff, ctx sumCtx) []SummaryLine {
	switch ctx.function {
	case "tpm2/quote", "tpm2/gettime":
		return generic(ds, func(p string) string {
			switch p {
			case "pcrdigest":
				return "Attested PCR digest (the value tpm2_attestedValue checks)"
			case "pcrselect":
				return "PCR selection"
			case "resetcount":
				return "TPM reset count (goes up when the TPM is reset: normally a reboot)"
			case "restartcount":
				return "TPM restart count (goes up on resume from suspend or hibernation)"
			case "firmwareVersion":
				return "TPM firmware version (the value tpm2_firmware checks)"
			case "qualifiedsigner":
				return "Signing key (qualified signer)"
			case "safe":
				return "TPM clock safe flag"
			}
			return p
		})
	case "uefi/efivars":
		return generic(ds, func(p string) string {
			name := strings.TrimPrefix(p, "variables.")
			if d := uefiVariableDescription(strings.SplitN(name, " ", 2)[0]); d != "" {
				return "UEFI variable " + name + " (" + d + ")"
			}
			return "UEFI variable " + name
		})
	case "uefi/eventlog":
		return summarizeEventLog(ds)
	case "ima/asciilog":
		return summarizeIMA(ds)
	}
	return generic(ds, plain)
}

func summarizeEventLog(ds []Diff) []SummaryLine {
	var out []SummaryLine
	var auth, events, counts, other []Diff
	for _, d := range ds {
		switch {
		case strings.HasPrefix(d.Path, "authorities."):
			auth = append(auth, d)
		case strings.HasPrefix(d.Path, "events."):
			events = append(events, d)
		case strings.HasPrefix(d.Path, "eventsperpcr."):
			counts = append(counts, d)
		default:
			other = append(other, d)
		}
	}
	out = append(out, generic(other, func(p string) string {
		if n := evKeyName(p); n != "" {
			return strings.ToUpper(n[:1]) + n[1:]
		}
		return p
	})...)
	for _, d := range auth {
		fp := strings.TrimPrefix(d.Path, "authorities.")
		if d.Op == "added" {
			out = append(out, line("A new boot authority (certificate that verified an image) "+fp, d))
		} else {
			out = append(out, line("Boot authority no longer used: "+fp, d))
		}
	}
	if len(events) > 0 {
		byPCR := map[string][]Diff{}
		for _, d := range events {
			pcr := strings.SplitN(strings.TrimPrefix(d.Path, "events."), " ", 2)[0]
			byPCR[pcr] = append(byPCR[pcr], d)
		}
		pcrs := make([]string, 0, len(byPCR))
		for p := range byPCR {
			pcrs = append(pcrs, p)
		}
		sort.Strings(pcrs)
		for _, p := range pcrs {
			es := byPCR[p]
			var ch, add, rem int
			var kinds []string
			seen := map[string]bool{}
			for _, d := range es {
				switch d.Op {
				case "changed":
					ch++
				case "added":
					add++
				case "removed":
					rem++
				}
				v := d.New
				if v == nil {
					v = d.Old
				}
				k := strings.SplitN(v.Text, " | ", 2)[0]
				if !seen[k] {
					seen[k] = true
					kinds = append(kinds, k)
				}
			}
			out = append(out, line(fmt.Sprintf("%s: %s (%s)", strings.Replace(p, "PCR", "PCR ", 1), counted(ch, add, rem, "event"), strings.Join(kinds, ", ")), es...))
		}
	}
	if len(counts) > 0 && len(events) == 0 {
		out = append(out, generic(counts, func(p string) string { return "Events in " + strings.TrimPrefix(p, "eventsperpcr.") })...)
	}
	return out
}

func counted(ch, add, rem int, noun string) string {
	var parts []string
	if ch > 0 {
		parts = append(parts, fmt.Sprintf("%d %s changed", ch, plural(ch, noun)))
	}
	if add > 0 {
		parts = append(parts, fmt.Sprintf("%d new", add))
	}
	if rem > 0 {
		parts = append(parts, fmt.Sprintf("%d gone", rem))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, s string) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

func summarizeIMA(ds []Diff) []SummaryLine {
	var out []SummaryLine
	var add, ch, rem, other []Diff
	for _, d := range ds {
		if !strings.HasPrefix(d.Path, "files.") {
			other = append(other, d)
			continue
		}
		switch d.Op {
		case "added":
			add = append(add, d)
		case "removed":
			rem = append(rem, d)
		default:
			ch = append(ch, d)
		}
	}
	for _, d := range other {
		switch d.Path {
		case "bootaggregate":
			out = append(out, line(describe("Boot aggregate (a new value means a new boot)", d), d))
		case "entries":
			out = append(out, line(describe("Entries in the IMA log", d), d))
		default:
			out = append(out, line(describe("IMA "+d.Path, d), d))
		}
	}
	files := func(ds []Diff) string {
		var ps []string
		for i, d := range ds {
			if i == 8 {
				ps = append(ps, fmt.Sprintf("and %d more", len(ds)-8))
				break
			}
			ps = append(ps, strings.TrimPrefix(d.Path, "files."))
		}
		return strings.Join(ps, ", ")
	}
	if len(ch) > 0 {
		out = append(out, line(fmt.Sprintf("%d %s measured with a different hash: %s", len(ch), plural(len(ch), "file"), files(ch)), ch...))
	}
	if len(add) > 0 {
		out = append(out, line(fmt.Sprintf("%d %s measured for the first time: %s", len(add), plural(len(add), "file"), files(add)), add...))
	}
	if len(rem) > 0 {
		out = append(out, line(fmt.Sprintf("%d %s no longer in the log (expected after a reboot): %s", len(rem), plural(len(rem), "file"), files(rem)), rem...))
	}
	return out
}

func summarizeElement(ds []Diff) []SummaryLine {
	var out []SummaryLine
	var rest []Diff
	profiles := map[string][]Diff{}
	for _, d := range ds {
		switch {
		case strings.HasPrefix(d.Path, "attestationprofiles."):
			name := strings.SplitN(strings.TrimPrefix(d.Path, "attestationprofiles."), ".", 2)[0]
			profiles[name] = append(profiles[name], d)
		case d.Path == "recordhistory.lastupdated" || d.Path == "recordhistory.created":
			out = append(out, line(fmt.Sprintf("Element %s: %s → %s", strings.TrimPrefix(d.Path, "recordhistory."), nsTime(d.Old), nsTime(d.New)), d))
		default:
			rest = append(rest, d)
		}
	}
	for _, name := range sortedKeys(profiles) {
		out = append(out, line(fmt.Sprintf("Attestation profile %s changed (%d values)", name, len(profiles[name])), profiles[name]...))
	}
	return append(out, generic(rest, func(p string) string {
		switch {
		case strings.HasPrefix(p, "tpm2.ek."):
			return "TPM EK " + strings.TrimPrefix(p, "tpm2.ek.") + " (a new EK means the TPM was reprovisioned or replaced)"
		case strings.HasPrefix(p, "tpm2.ak."):
			return "TPM AK " + strings.TrimPrefix(p, "tpm2.ak.") + " (a new AK means the TPM was reprovisioned)"
		case strings.HasPrefix(p, "endpoints."):
			return "Endpoint " + strings.TrimPrefix(p, "endpoints.")
		case strings.HasPrefix(p, "host."):
			return "Host " + strings.TrimPrefix(p, "host.")
		}
		return "Element " + p
	})...)
}

// nsTime writes a Jane timestamp value (nanoseconds) as a UTC time.
func nsTime(v *Value) string {
	if v == nil {
		return "—"
	}
	n, ok := atoi(v.Text)
	if !ok || n <= 0 {
		return v.Text
	}
	return time.Unix(0, n).UTC().Format("2006-01-02 15:04:05Z")
}
