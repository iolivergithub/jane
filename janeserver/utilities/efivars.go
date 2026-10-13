package utilities

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Vendor GUIDs of the UEFI variables Jane knows about
const (
	EfiGlobalVariableGUID        = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	EfiImageSecurityDatabaseGUID = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	ShimLockGUID                 = "605dab50-e046-4300-abb6-3dd810dd8b23"
)

// EfiVariable is one UEFI variable from the body of a uefi/efivars claim.
type EfiVariable struct {
	Name       string
	GUID       string // lower case
	Attributes uint32
	Size       int    // bytes read as reported by the agent, -1 if not reported
	Data       []byte // the value
	Lossy      bool   // the agent sent the value as a string and non-UTF-8 bytes were lost in transit
	Error      string // read error reported by the agent
}

// EfiVarsBody is the parsed body of a uefi/efivars claim.
type EfiVarsBody struct {
	Count     int
	Variables []EfiVariable
}

// Find returns the variable with the given name and vendor GUID, if it is present.
func (b EfiVarsBody) Find(name, guid string) (EfiVariable, bool) {
	guid = strings.ToLower(guid)
	for _, v := range b.Variables {
		if v.Name == name && v.GUID == guid {
			return v, true
		}
	}
	return EfiVariable{}, false
}

// ParseEfiVars reads the body of a uefi/efivars claim. It accepts the body as decoded
// from JSON or as read back from the database, so lists may be []interface{} or a named
// slice type, documents may be maps or ordered {Key, Value} lists, and numbers may be
// any numeric type.
//
// tarzan sends the bytes actually read, base64 encoded, as "rawvalue". Older agents only
// send "value", the bytes as a Go string, in which JSON encoding has replaced invalid
// UTF-8 with U+FFFD; such values are marked Lossy.
func ParseEfiVars(body map[string]interface{}) (EfiVarsBody, error) {
	raw, ok := body["efivars"]
	if !ok {
		return EfiVarsBody{}, fmt.Errorf("claim does not contain efivars")
	}
	items, ok := asSlice(raw)
	if !ok {
		return EfiVarsBody{}, fmt.Errorf("efivars in claim is not a list")
	}

	result := EfiVarsBody{}
	for _, item := range items {
		m := asMap(item)
		if m == nil {
			continue
		}
		result.Variables = append(result.Variables, makeEfiVariable(m))
	}

	if c, ok := asInt(body["count"]); ok {
		result.Count = c
	} else {
		result.Count = len(result.Variables)
	}
	return result, nil
}

func makeEfiVariable(m map[string]interface{}) EfiVariable {
	v := EfiVariable{Size: -1}
	v.Name, _ = m["name"].(string)
	guid, _ := m["guid"].(string)
	v.GUID = strings.ToLower(guid)
	v.Error, _ = m["errorvalue"].(string)
	if a, ok := asInt(m["attributes"]); ok {
		v.Attributes = uint32(a)
	}
	if s, ok := asInt(m["somevalue"]); ok && s >= 0 { // tarzan reports the number of bytes read as "somevalue"
		v.Size = s
	}

	if rv, ok := m["rawvalue"].(string); ok {
		if d, err := base64.StdEncoding.DecodeString(rv); err == nil {
			v.Data = d
			if v.Size < 0 || v.Size > len(d) {
				v.Size = len(d)
			}
			return v
		}
	}

	s, _ := m["value"].(string)
	if strings.ContainsRune(s, utf8.RuneError) || !utf8.ValidString(s) {
		// bytes were expanded to U+FFFD, so offsets no longer line up with the
		// reported size; only trailing padding can be removed
		v.Lossy = true
		v.Data = []byte(strings.TrimRight(s, "\x00"))
	} else {
		v.Data = []byte(s)
		if v.Size >= 0 && v.Size < len(v.Data) {
			v.Data = v.Data[:v.Size]
		}
	}
	if v.Size < 0 {
		v.Size = len(v.Data)
	}
	if v.Error != "" && v.Size == 0 {
		v.Data = nil
	}
	return v
}

func asSlice(x interface{}) ([]interface{}, bool) {
	if s, ok := x.([]interface{}); ok {
		return s, true
	}
	rv := reflect.ValueOf(x)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	out := make([]interface{}, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
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
			k, val := e.FieldByName("Key"), e.FieldByName("Value")
			if !k.IsValid() || !val.IsValid() || k.Kind() != reflect.String {
				return nil
			}
			out[k.String()] = val.Interface()
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
