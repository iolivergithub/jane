package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	loc := time.FixedZone("EET", 2*3600)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, loc)
	cases := []struct {
		in   string
		want time.Time
		date bool
	}{
		{"", now, false},
		{"now", now, false},
		{"36h", now.Add(-36 * time.Hour), false},
		{"7d", now.Add(-7 * 24 * time.Hour), false},
		{"2w", now.Add(-14 * 24 * time.Hour), false},
		{"90m", now.Add(-90 * time.Minute), false},
		{"2026-11-10", time.Date(2026, 11, 10, 0, 0, 0, 0, loc), true},
		{"2026-11-10T09:30", time.Date(2026, 11, 10, 9, 30, 0, 0, loc), false},
		{"2026-11-10 09:30:15", time.Date(2026, 11, 10, 9, 30, 15, 0, loc), false},
		{"2026-11-10T09:30:00Z", time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC), false},
		{"2026-11-10T09:30:00+05:00", time.Date(2026, 11, 10, 4, 30, 0, 0, time.UTC), false},
	}
	for _, c := range cases {
		got, date, err := parseTime(c.in, now)
		if err != nil || !got.Equal(c.want) || date != c.date {
			t.Errorf("parseTime(%q) = %v %v %v, want %v %v", c.in, got, date, err, c.want, c.date)
		}
	}
	for _, bad := range []string{"yesterday", "2026-13-01", "5y", "-3h"} {
		if _, _, err := parseTime(bad, now); err == nil {
			t.Errorf("parseTime(%q) should fail", bad)
		}
	}
}

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	r, err := parseRange("", "", now)
	if err != nil || !r.All {
		t.Fatalf("no times: want the whole history, got %+v %v", r, err)
	}
	// --from only: until now
	r, _ = parseRange("2026-11-10", "", now)
	if !r.From.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) || !r.Until.Equal(now) {
		t.Errorf("--from only: %v", r)
	}
	// --until only: --from is now, so the range is from --until to now
	r, _ = parseRange("", "2026-11-10T00:00:00Z", now)
	if !r.From.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) || !r.Until.Equal(now) {
		t.Errorf("--until only: %v", r)
	}
	// either order; a date as the later end is the whole day
	a, _ := parseRange("2026-11-12", "2026-11-10", now)
	b, _ := parseRange("2026-11-10", "2026-11-12", now)
	if !a.From.Equal(b.From) || !a.Until.Equal(b.Until) {
		t.Errorf("order matters: %v vs %v", a, b)
	}
	if want := time.Date(2026, 11, 12, 23, 59, 59, 999999999, time.UTC); !b.Until.Equal(want) {
		t.Errorf("date as later end: %v, want %v", b.Until, want)
	}
	if !b.contains(time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC).UnixNano()) || b.contains(time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC).UnixNano()) {
		t.Errorf("contains")
	}
	if _, err := parseRange("rubbish", "", now); err == nil || !strings.Contains(err.Error(), "--from") {
		t.Errorf("bad --from: %v", err)
	}
}

func TestParseArgs(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	o, err := parseArgs([]string{"--from", "7d", "E1", "--until=now", "--max", "50", "--nodocuments", "--out", "x.html"}, env(map[string]string{"JANE_SERVER": "http://jane:8520"}))
	if err != nil || o.elementID != "E1" || o.from != "7d" || o.until != "now" || o.max != 50 || !o.noDocuments || o.out != "x.html" || o.server != "http://jane:8520" {
		t.Errorf("parseArgs: %+v %v", o, err)
	}
	o, _ = parseArgs([]string{"E1", "--server", "http://a:1"}, env(map[string]string{"JANE_SERVER": "http://b:2"}))
	if o.server != "http://a:1" {
		t.Errorf("--server should win over JANE_SERVER")
	}
	for _, bad := range [][]string{{}, {"E1", "E2"}, {"E1", "--max", "0"}, {"E1", "--bogus"}, {"E1", "--from"}} {
		if _, err := parseArgs(append(bad, "--server", "http://x"), env(nil)); err == nil {
			t.Errorf("parseArgs(%v) should fail", bad)
		}
	}
	if _, err := parseArgs([]string{"E1"}, env(nil)); err == nil {
		t.Errorf("no server should fail")
	}
	if _, err := parseArgs([]string{"-h"}, env(nil)); err != errHelp {
		t.Errorf("-h: %v", err)
	}
}

func TestDiffValues(t *testing.T) {
	old := map[string]any{"a": "1", "list": []any{"x", "y"}, "m": map[string]any{}, "gone": "z"}
	new := map[string]any{"a": "2", "list": []any{"x", "y", "w"}, "m": map[string]any{"k": "v"}}
	ds := diffValues(old, new)
	got := map[string]string{}
	for _, d := range ds {
		got[d.Path] = d.Op
	}
	want := map[string]string{"a": "changed", "list[2]": "added", "m": "removed", "m.k": "added", "gone": "removed"}
	for p, op := range want {
		if got[p] != op {
			t.Errorf("%s: got %q, want %q (all: %v)", p, got[p], op, got)
		}
	}
	if len(diffValues(old, old)) != 0 {
		t.Errorf("a value differs from itself")
	}
	if ds := diffValues(nil, map[string]any{"a": "1"}); len(ds) != 1 || ds[0].Op != "added" {
		t.Errorf("from nothing: %+v", ds)
	}
}

func TestItemKeys(t *testing.T) {
	byName := []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}
	if k := itemKeys(byName); k[0] != "[name=a]" || k[1] != "[name=b]" {
		t.Errorf("by name: %v", k)
	}
	byGuid := []any{map[string]any{"name": "a", "guid": "1"}, map[string]any{"name": "a", "guid": "2"}}
	if k := itemKeys(byGuid); k[1] != "[name=a,guid=2]" {
		t.Errorf("by name and guid: %v", k)
	}
	mixed := []any{map[string]any{"name": "a"}, "b"}
	if k := itemKeys(mixed); k[1] != "[1]" {
		t.Errorf("by index: %v", k)
	}
}

func TestNaturalLessAndTruncation(t *testing.T) {
	if !naturalLess("sha256.7", "sha256.10") || naturalLess("sha256.10", "sha256.7") || !naturalLess("a[2]", "a[10]") {
		t.Errorf("natural order")
	}
	long := strings.Repeat("é", maxValueText) // 2 bytes each
	v := makeValue(long)
	if !v.Truncated || v.Len != len(long) || len(v.SHA256) != 64 || len(v.Text) > maxValueText || !strings.HasPrefix(long, v.Text) {
		t.Errorf("truncation: %+v", v)
	}
	if v := makeValue("short"); v.Truncated || v.Text != "short" {
		t.Errorf("short value changed")
	}
}

func TestBigTimestampsExact(t *testing.T) {
	d, err := parseDoc([]byte(`{"t": 1763370000123456789}`))
	if err != nil || d.num("t") != 1763370000123456789 {
		t.Errorf("timestamp not exact: %d %v", d.num("t"), err)
	}
}

func TestAPIKey(t *testing.T) {
	good := "jane_" + strings.Repeat("A", 43)
	env := func(k string) string { return map[string]string{"JANE_API_KEY": good}[k] }
	if k, err := apiKey(nil, env, nil); err != nil || k != good {
		t.Errorf("from env: %q %v", k, err)
	}
	file := "/k"
	read := func(string) ([]byte, error) { return []byte("bad\n"), nil }
	if _, err := apiKey(&file, env, read); err == nil {
		t.Errorf("a bad key in the file should be refused")
	}
	if insecureRemote("http://localhost:8520") || !insecureRemote("http://jane:8520") || insecureRemote("https://jane:8520") {
		t.Errorf("insecureRemote")
	}
}
