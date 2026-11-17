package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// maxValueText is the most of a value kept in the report; longer values are
// shortened and carry their length and SHA-256 instead.
const maxValueText = 1024

// Value is one side of a difference.
type Value struct {
	Text      string `json:"t"`
	Len       int    `json:"n,omitempty"` // length of the whole value, if shortened
	SHA256    string `json:"h,omitempty"` // of the whole value, if shortened
	Truncated bool   `json:"x,omitempty"`
}

// Diff is one changed leaf: Op is added, removed or changed.
type Diff struct {
	Path string `json:"p"`
	Op   string `json:"o"`
	Old  *Value `json:"a,omitempty"`
	New  *Value `json:"b,omitempty"`
}

// diffValues compares two JSON values leaf by leaf.
func diffValues(old, new any) []Diff {
	a, b := map[string]string{}, map[string]string{}
	flatten(old, "", a)
	flatten(new, "", b)
	var out []Diff
	for p, av := range a {
		bv, ok := b[p]
		switch {
		case !ok:
			out = append(out, Diff{Path: p, Op: "removed", Old: makeValue(av)})
		case av != bv:
			out = append(out, Diff{Path: p, Op: "changed", Old: makeValue(av), New: makeValue(bv)})
		}
	}
	for p, bv := range b {
		if _, ok := a[p]; !ok {
			out = append(out, Diff{Path: p, Op: "added", New: makeValue(bv)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return naturalLess(out[i].Path, out[j].Path) })
	return out
}

func makeValue(s string) *Value {
	if len(s) <= maxValueText {
		return &Value{Text: s}
	}
	h := sha256.Sum256([]byte(s))
	cut := maxValueText
	for cut > 0 && !utf8Start(s[cut]) {
		cut--
	}
	return &Value{Text: s[:cut], Len: len(s), SHA256: hex.EncodeToString(h[:]), Truncated: true}
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// flatten writes the leaves of v into out, keyed by path: object keys joined
// with ".", array items as [i]. An array of objects that all have a distinct
// "name" is keyed by name instead, [name=...] (with the guid as well where
// names repeat, as for UEFI variables), so that an item moving within the
// array is not reported as a change. Strings are kept as they are, anything
// else as JSON; empty objects and arrays are leaves {} and [].
func flatten(v any, path string, out map[string]string) {
	switch t := v.(type) {
	case nil:
		if path != "" {
			out[path] = "null"
		}
	case map[string]any:
		if len(t) == 0 {
			out[orRoot(path)] = "{}"
			return
		}
		for k, x := range t {
			flatten(x, join(path, k), out)
		}
	case []any:
		if len(t) == 0 {
			out[orRoot(path)] = "[]"
			return
		}
		keys := itemKeys(t)
		for i, x := range t {
			flatten(x, path+keys[i], out)
		}
	case string:
		out[orRoot(path)] = t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			b = []byte(fmt.Sprint(t))
		}
		out[orRoot(path)] = string(b)
	}
}

func orRoot(p string) string {
	if p == "" {
		return "(value)"
	}
	return p
}

func join(path, k string) string {
	if path == "" {
		return k
	}
	return path + "." + k
}

// itemKeys names the items of an array: by name (and guid) when that
// identifies every item, else by index.
func itemKeys(items []any) []string {
	keys := make([]string, len(items))
	for _, by := range [][]string{{"name"}, {"name", "guid"}} {
		seen := map[string]bool{}
		ok := true
		for i, x := range items {
			m, isMap := x.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			var parts []string
			for _, f := range by {
				s, isStr := m[f].(string)
				if !isStr || s == "" {
					ok = false
					break
				}
				parts = append(parts, f+"="+s)
			}
			if !ok {
				break
			}
			k := "[" + strings.Join(parts, ",") + "]"
			if seen[k] {
				ok = false
				break
			}
			seen[k] = true
			keys[i] = k
		}
		if ok {
			return keys
		}
	}
	for i := range items {
		keys[i] = fmt.Sprintf("[%d]", i)
	}
	return keys
}

// naturalLess orders paths with their numbers by value, so that sha256.7
// comes before sha256.10.
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			na, ra := leadingDigits(a)
			nb, rb := leadingDigits(b)
			ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			a, b = ra, rb
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c < 0x80 && unicode.IsDigit(rune(c)) }

func leadingDigits(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// pretty indents a JSON document without changing its numbers.
func pretty(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
