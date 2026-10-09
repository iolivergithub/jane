package x3270

// Screen layout for a standard 24 x 80 3270 display.
//
//	row 0      title (left) and position / time (right)
//	row 1      Command ===> input
//	row 3      column headings (lists)
//	rows 4-20  list rows, or rows 3-20 of text (detail screens)
//	row 22     message line
//	row 23     function key legend
//
// A field's attribute byte occupies its column, so a field at column 0
// displays its content from column 1 to 79: 79 usable characters per line.

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/racingmars/go3270"

	"a10/structures"
)

const (
	screenRows = 24
	lineWidth  = 79 // characters available to a field starting at column 0

	cmdRow   = 1
	cmdCol   = 13 // attribute byte of the command input field
	headRow  = 3
	firstRow = 4  // first list row
	lastRow  = 20 // last list / text row
	msgRow   = 22
	keysRow  = 23

	listRows = lastRow - firstRow + 1 // rows per list page
	textRows = lastRow - headRow + 1  // rows per detail page

	selCol    = 0 // attribute byte of a list row's selection field
	rowCol    = 2 // attribute byte of a list row's text
	rowWidth  = 79 - rowCol
	timeStamp = "2006-01-02 15:04:05"
)

// clean replaces anything a 3270 terminal can't display (control characters,
// non-ASCII) so text never disturbs the layout.
func clean(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			b.WriteByte('.')
		case r < 0x7f:
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

// fit cleans s and pads or truncates it to exactly width characters.
func fit(s string, width int) string {
	s = clean(s)
	if n := utf8.RuneCountInString(s); n > width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// cut cleans s and truncates it to at most width characters, marking the cut.
func cut(s string, width int) string {
	s = clean(s)
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + ">"
}

// wrap cleans s and splits it into lines of at most width characters,
// breaking at spaces where possible. Embedded newlines start new lines.
func wrap(s string, width int) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		para = clean(para)
		if para == "" {
			out = append(out, "")
			continue
		}
		for len(para) > width {
			brk := strings.LastIndex(para[:width+1], " ")
			if brk <= width/2 {
				brk = width
			}
			out = append(out, strings.TrimRight(para[:brk], " "))
			para = strings.TrimLeft(para[brk:], " ")
		}
		out = append(out, para)
	}
	return out
}

// labelled lays out "label: value", wrapping the value under itself.
func labelled(label, value string) []string {
	const lw = 16
	prefix := fit(label, lw) + " "
	lines := wrap(value, lineWidth-len(prefix))
	if len(lines) == 0 {
		lines = []string{""}
	}
	out := []string{prefix + lines[0]}
	for _, l := range lines[1:] {
		out = append(out, strings.Repeat(" ", len(prefix))+l)
	}
	return out
}

// flatten turns nested maps and lists into sorted "key: value" lines.
func flatten(prefix string, v interface{}) []string {
	var out []string
	switch t := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			name := k
			if prefix != "" {
				name = prefix + "." + k
			}
			out = append(out, flatten(name, t[k])...)
		}
	case []interface{}:
		for i, e := range t {
			out = append(out, flatten(fmt.Sprintf("%s[%d]", prefix, i), e)...)
		}
	default:
		out = append(out, labelled(prefix, fmt.Sprintf("%v", t))...)
	}
	return out
}

func when(t structures.Timestamp) string {
	if t == 0 {
		return "-"
	}
	return time.Unix(0, int64(t)).UTC().Format(timeStamp)
}

func took(from, to structures.Timestamp) string {
	if from == 0 || to == 0 || to < from {
		return "-"
	}
	d := time.Duration(to - from)
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(10 * time.Millisecond).String()
	default:
		return d.Round(time.Second).String()
	}
}

// resultLabel is the readable name of a result value.
func resultLabel(v structures.ResultValue) string {
	switch v {
	case structures.Success:
		return "Pass"
	case structures.Fail:
		return "Fail"
	case structures.VerifyCallFailure:
		return "Verify call failure"
	case structures.VerifyClaimErrorAttempt:
		return "Verify claim error"
	case structures.NoResult:
		return "No result"
	case structures.MissingExpectedValue:
		return "Missing expected value"
	case structures.RuleCallFailure:
		return "Rule call failure"
	case structures.UnsetResultValue:
		return "Unset result value"
	default:
		return "Unknown"
	}
}

// resultColor colours one result: green pass, red fail, yellow anything else.
func resultColor(v structures.ResultValue) go3270.Color {
	switch v {
	case structures.Success:
		return go3270.Green
	case structures.Fail:
		return go3270.Red
	default:
		return go3270.Yellow
	}
}

// frame is the part every screen shares: title, command line, message line
// and function key legend.
func frame(title, right, message, keys string) go3270.Screen {
	return go3270.Screen{
		{Row: 0, Col: 0, Intense: true, Color: go3270.White, Content: fit("JANE  "+title, lineWidth-len(right)-1) + " " + clean(right)},
		{Row: cmdRow, Col: 0, Color: go3270.Green, Content: "Command ===>"},
		{Row: cmdRow, Col: cmdCol, Write: true, Name: "cmd", Highlighting: go3270.Underscore, Color: go3270.Turquoise},
		{Row: cmdRow, Col: 79, Autoskip: true}, // ends the command field
		{Row: msgRow, Col: 0, Intense: true, Color: go3270.Red, Content: cut(message, lineWidth)},
		{Row: keysRow, Col: 0, Color: go3270.Turquoise, Content: cut(keys, lineWidth)},
	}
}
