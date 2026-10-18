package webui

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/google/go-eventlog/tcg"

	"a10/utilities"
)

// EventLogEntry is one event of a uefi/eventlog claim, prepared for display.
type EventLogEntry struct {
	Num         int
	PCR         int
	Type        string
	Digest      string // hex digest in this bank, "" if the event has none
	ShortDigest string // first 16 hex digits, for the table
	Description string // light decoding of the event data (#73)
}

// EventLogBankView is the events of one bank (digest algorithm) of the log.
type EventLogBankView struct {
	Name   string // sha1, sha256, sha384
	Events []EventLogEntry
}

// EventLogView is the body of a uefi/eventlog claim, prepared for display.
type EventLogView struct {
	Error       string
	Count       int
	PCRs        []int // PCRs the log extends
	Banks       []EventLogBankView
	DefaultBank string // shown first: sha256 if present
}

// EventLogBody turns the body of a uefi/eventlog claim into something a template can render.
func EventLogBody(body map[string]interface{}) EventLogView {
	view := EventLogView{}

	b64, ok := body["eventlog"].(string)
	if !ok {
		view.Error = "the claim has no eventlog field"
		return view
	}
	el, err := utilities.DecodeEventLog(b64)
	if err != nil {
		view.Error = err.Error()
		return view
	}

	pcrs := map[int]bool{}
	for name, alg := range utilities.EventLogBanks(el) {
		bank := EventLogBankView{Name: name}
		for i, e := range el.Events(alg) {
			typ := e.Type // not UntrustedType(): in go-eventlog v0.0.2 it panics on unknown types
			if typ.String() != "EV_NO_ACTION" {
				pcrs[e.Index] = true
			}
			digest := hex.EncodeToString(e.ReplayedDigest())
			short := digest
			if len(short) > 16 {
				short = short[:16] + "…"
			}
			bank.Events = append(bank.Events, EventLogEntry{
				Num:         i + 1, // Events does not number the events
				PCR:         e.Index,
				Type:        typ.String(),
				Digest:      digest,
				ShortDigest: short,
				Description: describeEvent(typ.String(), e.RawData()),
			})
		}
		view.Count = len(bank.Events)
		view.Banks = append(view.Banks, bank)
	}

	sort.Slice(view.Banks, func(i, j int) bool { return view.Banks[i].Name < view.Banks[j].Name })
	for _, b := range view.Banks {
		if b.Name == "sha256" || view.DefaultBank == "" {
			view.DefaultBank = b.Name
		}
	}
	for p := range pcrs {
		view.PCRs = append(view.PCRs, p)
	}
	sort.Ints(view.PCRs)
	return view
}

// describeEvent decodes a little of an event's data: the UEFI variable for
// variable events, the file path for image loads, text where the data is
// text, otherwise its size. Deeper decoding is #75.
func describeEvent(typ string, data []byte) string {
	switch {
	case strings.HasPrefix(typ, "EV_EFI_VARIABLE_"):
		v, err := tcg.ParseUEFIVariableData(bytes.NewReader(data))
		if err != nil {
			break
		}
		guid := v.Header.VariableName.String()
		if name, ok := efiKnownGUIDs[guid]; ok {
			guid = name
		}
		return fmt.Sprintf("%s (%s), %d bytes", v.VarName(), guid, len(v.VariableData))

	case typ == "EV_EFI_BOOT_SERVICES_APPLICATION", typ == "EV_EFI_BOOT_SERVICES_DRIVER", typ == "EV_EFI_RUNTIME_SERVICES_DRIVER":
		img, err := tcg.ParseEFIImageLoad(bytes.NewReader(data))
		if err != nil {
			break
		}
		if path := imageFilePath(&img); path != "" {
			return fmt.Sprintf("%s, %d bytes", path, img.Header.Length)
		}
		return fmt.Sprintf("image, %d bytes", img.Header.Length)
	}

	if s, ok := printableText(data); ok {
		return s
	}
	return fmt.Sprintf("%d bytes", len(data))
}

// imageFilePath joins the file path nodes (media device, subtype 4) of an
// image load's device path.
func imageFilePath(img *tcg.EFIImageLoad) string {
	elems, err := img.DevicePath()
	if err != nil {
		return ""
	}
	var parts []string
	for _, e := range elems {
		if e.Type != tcg.MediaDevice || e.Subtype != 4 || len(e.Data)%2 != 0 {
			continue
		}
		u := make([]uint16, len(e.Data)/2)
		for i := range u {
			u[i] = uint16(e.Data[2*i]) | uint16(e.Data[2*i+1])<<8
		}
		parts = append(parts, strings.TrimRight(string(utf16.Decode(u)), "\x00"))
	}
	return strings.Join(parts, "")
}

// maxEventText limits the text shown for one event.
const maxEventText = 200

// printableText returns data as text if it is printable ASCII, ignoring
// trailing NULs.
func printableText(data []byte) (string, bool) {
	data = bytes.TrimRight(data, "\x00")
	if len(data) == 0 {
		return "", false
	}
	for _, c := range data {
		if (c < 0x20 || c > 0x7e) && c != '\t' && c != '\n' && c != '\r' {
			return "", false
		}
	}
	s := string(data)
	if len(s) > maxEventText {
		s = s[:maxEventText] + "…"
	}
	return s, true
}
