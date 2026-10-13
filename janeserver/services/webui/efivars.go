package webui

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"a10/rules/uefirules"
	"a10/utilities"
)

// EfiVar is one UEFI variable from a uefi/efivars claim, prepared for display.
type EfiVar struct {
	Name           string
	GUID           string
	GUIDName       string   // well-known name of the vendor GUID, if any
	Attributes     uint32   // raw attribute bitmask
	AttributeFlags []string // short names of the set attribute bits
	Size           int      // number of bytes in the value
	Decoded        string   // interpretation of the value for well-known variables, if any
	Text           string   // value as text, if it looks like UCS-2 or ASCII text
	HexDump        string   // offset / hex / ASCII dump of the value
	EVValue        string   // the value as the UEFI_ rules expect it in an expected value
	RuleName       string   // the UEFI_ rule that checks this variable, if any
	Error          string
	Lossy          bool // the value was sent as a string and non-UTF-8 bytes were lost in transit
}

// EfiVarsView is the body of a uefi/efivars claim, prepared for display.
type EfiVarsView struct {
	Count    int
	Vars     []EfiVar
	Errors   int
	AnyLossy bool
}

var efiKnownGUIDs = map[string]string{
	utilities.EfiGlobalVariableGUID:        "EFI Global Variable",
	utilities.EfiImageSecurityDatabaseGUID: "Image Security Database",
	utilities.ShimLockGUID:                 "Shim",
	"4a67b082-0a4c-41cf-b6c7-440b29bb8c4f": "systemd-boot / Loader",
	"eb704011-1402-11d3-8e77-00a0c969723b": "MTC Vendor",
	"77fa9abd-0359-4d32-bd60-28f4e78f784b": "Microsoft",
}

var efiAttributeBits = []struct {
	bit  uint32
	name string
	desc string
}{
	{0x01, "NV", "Non-volatile"},
	{0x02, "BS", "Boot service access"},
	{0x04, "RT", "Runtime access"},
	{0x08, "HW", "Hardware error record"},
	{0x10, "AW", "Authenticated write access (deprecated)"},
	{0x20, "TA", "Time-based authenticated write access"},
	{0x40, "AP", "Append write"},
	{0x80, "EA", "Enhanced authenticated access"},
}

// EfiAttributeDescription is the long name of an attribute flag, for tooltips.
func EfiAttributeDescription(flag string) string {
	for _, a := range efiAttributeBits {
		if a.name == flag {
			return a.desc
		}
	}
	return flag
}

// EfiVarsBody turns the body of a uefi/efivars claim into something a template can render.
func EfiVarsBody(body map[string]interface{}) EfiVarsView {
	view := EfiVarsView{}

	parsed, err := utilities.ParseEfiVars(body)
	if err != nil {
		return view
	}
	view.Count = parsed.Count

	for _, ev := range parsed.Variables {
		v := makeEfiVar(ev)
		if v.Error != "" {
			view.Errors++
		}
		if v.Lossy {
			view.AnyLossy = true
		}
		view.Vars = append(view.Vars, v)
	}

	// well-known vendor GUIDs first, then by GUID, then by name
	sort.SliceStable(view.Vars, func(i, j int) bool {
		a, b := view.Vars[i], view.Vars[j]
		if (a.GUIDName != "") != (b.GUIDName != "") {
			return a.GUIDName != ""
		}
		if a.GUID != b.GUID {
			return a.GUID < b.GUID
		}
		return a.Name < b.Name
	})

	return view
}

// efiEVInlineLimit is the largest value the claim view offers as hex for an expected value;
// anything longer is offered as its SHA-256 (the UEFI_ rules accept either form).
const efiEVInlineLimit = 32

func makeEfiVar(e utilities.EfiVariable) EfiVar {
	v := EfiVar{
		Name:       e.Name,
		GUID:       e.GUID,
		GUIDName:   efiKnownGUIDs[e.GUID],
		Attributes: e.Attributes,
		Size:       e.Size,
		Error:      e.Error,
		Lossy:      e.Lossy,
	}
	for _, b := range efiAttributeBits {
		if v.Attributes&b.bit != 0 {
			v.AttributeFlags = append(v.AttributeFlags, b.name)
		}
	}
	for _, r := range uefirules.Variables {
		if r.Name == v.Name && r.GUID == v.GUID {
			v.RuleName = uefirules.RuleName(r.Name)
		}
	}

	data := e.Data
	if len(data) == 0 {
		return v
	}
	if !v.Lossy {
		v.Decoded = decodeKnownEfiVar(v.Name, v.GUID, data)
		if len(data) <= efiEVInlineLimit {
			v.EVValue = hex.EncodeToString(data)
		} else {
			h := sha256.Sum256(data)
			v.EVValue = "sha256:" + hex.EncodeToString(h[:])
		}
	}
	v.Text = efiValueAsText(data)
	v.HexDump = hexDump(data)
	return v
}

// decodeKnownEfiVar interprets the handful of well-known variables whose formats are simple.
func decodeKnownEfiVar(name, guid string, data []byte) string {
	if guid != utilities.EfiGlobalVariableGUID {
		return ""
	}
	switch name {
	case "SecureBoot", "SetupMode", "AuditMode", "DeployedMode", "VendorKeys":
		if len(data) == 1 {
			if data[0] == 1 {
				return "Enabled (1)"
			}
			if data[0] == 0 {
				return "Disabled (0)"
			}
			return fmt.Sprintf("%d", data[0])
		}
	case "BootCurrent", "BootNext":
		if len(data) == 2 {
			return fmt.Sprintf("Boot%04X", binary.LittleEndian.Uint16(data))
		}
	case "Timeout":
		if len(data) == 2 {
			return fmt.Sprintf("%d seconds", binary.LittleEndian.Uint16(data))
		}
	case "BootOrder", "DriverOrder", "SysPrepOrder":
		if len(data) > 0 && len(data)%2 == 0 {
			prefix := strings.TrimSuffix(name, "Order")
			parts := make([]string, 0, len(data)/2)
			for i := 0; i < len(data); i += 2 {
				parts = append(parts, fmt.Sprintf("%s%04X", prefix, binary.LittleEndian.Uint16(data[i:])))
			}
			return strings.Join(parts, ", ")
		}
	case "PlatformLang", "Lang", "PlatformLangCodes", "LangCodes":
		return strings.TrimRight(string(data), "\x00")
	}

	// EFI_LOAD_OPTION: Attributes (u32), FilePathListLength (u16), Description (UCS-2, NUL terminated), ...
	if isLoadOptionName(name) && len(data) >= 8 {
		attrs := binary.LittleEndian.Uint32(data)
		desc, ok := ucs2Prefix(data[6:])
		if ok {
			active := "inactive"
			if attrs&0x1 != 0 {
				active = "active"
			}
			return fmt.Sprintf("%q (%s)", desc, active)
		}
	}
	return ""
}

func isLoadOptionName(name string) bool {
	for _, p := range []string{"Boot", "Driver", "SysPrep"} {
		if len(name) == len(p)+4 && strings.HasPrefix(name, p) {
			for _, c := range name[len(p):] {
				if !strings.ContainsRune("0123456789ABCDEFabcdef", c) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// ucs2Prefix reads a NUL-terminated UCS-2LE string from the start of b.
func ucs2Prefix(b []byte) (string, bool) {
	var u []uint16
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			return string(utf16.Decode(u)), true
		}
		u = append(u, c)
	}
	return "", false
}

// efiValueAsText returns the value as text when it is clearly UCS-2LE or ASCII text, otherwise "".
func efiValueAsText(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	if len(b)%2 == 0 {
		u := make([]uint16, 0, len(b)/2)
		for i := 0; i < len(b); i += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[i:]))
		}
		for len(u) > 0 && u[len(u)-1] == 0 {
			u = u[:len(u)-1]
		}
		if len(u) > 0 && allPrintable(utf16.Decode(u)) {
			return string(utf16.Decode(u))
		}
	}
	s := strings.TrimRight(string(b), "\x00")
	if len(s) >= 2 && utf8.ValidString(s) && allPrintable([]rune(s)) {
		return s
	}
	return ""
}

func allPrintable(rs []rune) bool {
	for _, r := range rs {
		if r == utf8.RuneError || !(unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r') {
			return false
		}
	}
	return true
}

// hexDump formats bytes as lines of "offset  hex bytes  |ascii|".
func hexDump(b []byte) string {
	var sb strings.Builder
	for off := 0; off < len(b); off += 16 {
		end := off + 16
		if end > len(b) {
			end = len(b)
		}
		fmt.Fprintf(&sb, "%04x  ", off)
		for i := off; i < off+16; i++ {
			if i < end {
				fmt.Fprintf(&sb, "%02x ", b[i])
			} else {
				sb.WriteString("   ")
			}
			if i == off+7 {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(" |")
		for _, c := range b[off:end] {
			if c >= 0x20 && c < 0x7f {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|")
		if end < len(b) {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
