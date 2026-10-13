package webui

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
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
	"8be4df61-93ca-11d2-aa0d-00e098032b8c": "EFI Global Variable",
	"d719b2cb-3d3a-4596-a3bc-dad00e67656f": "Image Security Database",
	"605dab50-e046-4300-abb6-3dd810dd8b23": "Shim",
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
// It accepts the body as decoded from JSON or read back from the database, so lists may be
// []interface{} or a named slice type, and numbers may be any numeric type.
func EfiVarsBody(body map[string]interface{}) EfiVarsView {
	view := EfiVarsView{}

	for _, item := range asSlice(body["efivars"]) {
		m := asMap(item)
		if m == nil {
			continue
		}
		v := makeEfiVar(m)
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

	if c, ok := asInt(body["count"]); ok {
		view.Count = c
	} else {
		view.Count = len(view.Vars)
	}
	return view
}

func makeEfiVar(m map[string]interface{}) EfiVar {
	v := EfiVar{}
	v.Name, _ = m["name"].(string)
	v.GUID, _ = m["guid"].(string)
	v.GUIDName = efiKnownGUIDs[strings.ToLower(v.GUID)]
	v.Error, _ = m["errorvalue"].(string)

	if a, ok := asInt(m["attributes"]); ok {
		v.Attributes = uint32(a)
		for _, b := range efiAttributeBits {
			if v.Attributes&b.bit != 0 {
				v.AttributeFlags = append(v.AttributeFlags, b.name)
			}
		}
	}

	// tarzan reports the number of bytes read as "somevalue"
	size, haveSize := asInt(m["somevalue"])

	var data []byte
	if raw, ok := m["rawvalue"].(string); ok && raw != "" {
		if d, err := base64.StdEncoding.DecodeString(raw); err == nil {
			data = d
		}
	}
	if data == nil {
		// Older agents send the raw bytes as a Go string; JSON encoding replaced any
		// invalid UTF-8 with U+FFFD, so this is only an approximation of the value.
		s, _ := m["value"].(string)
		if !utf8.ValidString(s) || strings.ContainsRune(s, utf8.RuneError) {
			v.Lossy = true
		}
		data = []byte(s)
	}

	// The agent's buffer may be larger than the value; trim it to the bytes actually read.
	// A lossy value has had bytes expanded to U+FFFD, so byte offsets no longer line up
	// and only trailing padding can be removed.
	if v.Lossy {
		data = []byte(strings.TrimRight(string(data), "\x00"))
	} else if haveSize && size >= 0 && size < len(data) {
		data = data[:size]
	}
	if v.Error != "" && !haveSize {
		data = nil
	}

	v.Size = len(data)
	if haveSize && size >= 0 {
		v.Size = size
	}
	if v.Error == "" || size > 0 {
		if !v.Lossy {
			v.Decoded = decodeKnownEfiVar(v.Name, strings.ToLower(v.GUID), data)
		}
		v.Text = efiValueAsText(data)
		v.HexDump = hexDump(data)
	}
	return v
}

const efiGlobalGUID = "8be4df61-93ca-11d2-aa0d-00e098032b8c"

// decodeKnownEfiVar interprets the handful of well-known variables whose formats are simple.
func decodeKnownEfiVar(name, guid string, data []byte) string {
	if guid != efiGlobalGUID {
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

func asSlice(x interface{}) []interface{} {
	if x == nil {
		return nil
	}
	if s, ok := x.([]interface{}); ok {
		return s
	}
	rv := reflect.ValueOf(x)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]interface{}, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

func asMap(x interface{}) map[string]interface{} {
	if m, ok := x.(map[string]interface{}); ok {
		return m
	}
	rv := reflect.ValueOf(x)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil
		}
		out := make(map[string]interface{}, rv.Len())
		for _, k := range rv.MapKeys() {
			out[k.String()] = rv.MapIndex(k).Interface()
		}
		return out
	case reflect.Slice:
		// an ordered document (e.g. bson.D): a slice of {Key, Value} structs
		out := make(map[string]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			e := reflect.Indirect(rv.Index(i))
			if e.Kind() != reflect.Struct {
				return nil
			}
			k, v := e.FieldByName("Key"), e.FieldByName("Value")
			if !k.IsValid() || !v.IsValid() || k.Kind() != reflect.String {
				return nil
			}
			out[k.String()] = v.Interface()
		}
		return out
	}
	return nil
}

func asInt(x interface{}) (int, bool) {
	rv := reflect.ValueOf(x)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return int(rv.Float()), true
	}
	return 0, false
}
