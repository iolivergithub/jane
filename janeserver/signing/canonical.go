package signing

// Canonical JSON serialisation, following RFC 8785 (JSON Canonicalization Scheme, JCS)
// with one deliberate deviation:
//
//   - Integer literals are written exactly as they are, not converted to an IEEE-754 double.
//     Jane's timestamps are int64 nanoseconds (~1.8e18), well beyond 2^53, and strict JCS
//     would round them so that different timestamps could canonicalise identically.
//
// Everything else is as RFC 8785: object members sorted by the UTF-16 code units of their
// names, no insignificant whitespace, ECMAScript string escaping, and non-integer numbers
// in ECMAScript Number.prototype.toString form.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

var integerLiteral = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// Canonicalise returns the canonical JSON bytes of v. v is first marshalled with
// encoding/json, so json struct tags apply and []byte values appear as base64 strings.
func Canonicalise(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonicalise: json marshal failed: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonicalise: json decode failed: %w", err)
	}

	var buf bytes.Buffer
	if err := writeCanonical(&buf, generic); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		s, err := canonicalNumber(x)
		if err != nil {
			return err
		}
		buf.WriteString(s)
	case string:
		writeString(buf, x)
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeString(buf, k)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("canonicalise: unexpected type %T", v)
	}
	return nil
}

// lessUTF16 orders strings by their UTF-16 code units, as RFC 8785 section 3.2.3 requires.
func lessUTF16(a, b string) bool {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// writeString escapes as ECMAScript JSON.stringify does: only '"', '\' and control characters.
func writeString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

func canonicalNumber(n json.Number) (string, error) {
	s := n.String()
	if integerLiteral.MatchString(s) {
		if s == "-0" {
			return "0", nil
		}
		return s, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", fmt.Errorf("canonicalise: number %q cannot be represented", s)
	}
	return formatES6(f), nil
}

// formatES6 renders a finite float64 as ECMAScript Number.prototype.toString does.
func formatES6(f float64) string {
	if f == 0 {
		return "0"
	}
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}

	// Shortest round-tripping digits, e.g. "1.2345e+06"
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expS, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expS)
	k := len(digits)
	n := exp + 1 // value = 0.digits * 10^n

	var out string
	switch {
	case k <= n && n <= 21:
		out = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		out = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		out = "0." + strings.Repeat("0", -n) + digits
	default:
		esign := "+"
		if n-1 < 0 {
			esign = "-"
		}
		eabs := n - 1
		if eabs < 0 {
			eabs = -eabs
		}
		if k == 1 {
			out = digits + "e" + esign + strconv.Itoa(eabs)
		} else {
			out = digits[:1] + "." + digits[1:] + "e" + esign + strconv.Itoa(eabs)
		}
	}
	return sign + out
}
