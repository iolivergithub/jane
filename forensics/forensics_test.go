package main

import (
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func findEvent(r *Report, id string) *Event {
	for i := range r.Events {
		if r.Events[i].ID == id {
			return &r.Events[i]
		}
	}
	return nil
}

func changeSet(e *Event, what string) *ChangeSet {
	if e == nil {
		return nil
	}
	for i := range e.Changes {
		if e.Changes[i].What == what {
			return &e.Changes[i]
		}
	}
	return nil
}

func diffAt(cs *ChangeSet, path string) *Diff {
	if cs == nil {
		return nil
	}
	for i := range cs.Diffs {
		if cs.Diffs[i].Path == path {
			return &cs.Diffs[i]
		}
	}
	return nil
}

func TestWholeHistory(t *testing.T) {
	f := story()
	now := time.Now()
	c, r := runStory(t, f, timeRange{All: true}, now)

	if len(c.claims) != 24 || len(c.results) != 24 || len(c.baseClaims) != 0 {
		t.Fatalf("claims %d results %d baselines %d, want 24, 24, 0", len(c.claims), len(c.results), len(c.baseClaims))
	}
	if len(c.sessions) != 12 || len(c.evs) != 2 || len(c.intents) != 2 {
		t.Fatalf("sessions %d evs %d intents %d, want 12, 2, 2", len(c.sessions), len(c.evs), len(c.intents))
	}
	for _, e := range r.Events {
		if strings.Contains(e.ID, "CX") || e.ItemID == "LX" || e.ItemID == "LI" || e.ItemID == "EVX" {
			t.Errorf("another element's item, or an intent's log entry, in the report: %s", e.ID)
		}
	}
	if findEvent(r, "log:LE1") == nil {
		t.Errorf("the element's own log entry is missing")
	}

	// PCR 7 changed at h(36), the 7th claim of the series
	cs := changeSet(findEvent(r, "claim:CP07"), "body")
	if d := diffAt(cs, "sha256.7"); d == nil || d.Op != "changed" || d.Old.Text != strings.Repeat("7", 64) || d.New.Text != strings.Repeat("a", 64) {
		t.Fatalf("PCR 7 change not found on CP07: %+v", cs)
	}
	if len(cs.Diffs) != 1 || cs.Against != "CP06" {
		t.Errorf("CP07 body: want only sha256.7 against CP06, got %+v", cs)
	}
	if changeSet(findEvent(r, "claim:CP08"), "body") != nil {
		t.Errorf("CP08 body did not change")
	}
	// SecureBoot changed at h(48)
	if d := diffAt(changeSet(findEvent(r, "claim:CE09"), "body"), "efivars[name=SecureBoot].value"); d == nil || d.Old.Text != "1" || d.New.Text != "0" {
		t.Errorf("SecureBoot change not found on CE09")
	}
	// a variable added at the front at h(60): additions only, matched by name
	cs = changeSet(findEvent(r, "claim:CE11"), "body")
	if cs == nil {
		t.Fatalf("no body change on CE11")
	}
	for _, d := range cs.Diffs {
		if d.Op == "changed" && d.Path != "count" {
			t.Errorf("CE11: moved variables reported as changed: %+v", d)
		}
	}
	if diffAt(cs, "efivars[name=MokListRT].value") == nil {
		t.Errorf("CE11: MokListRT addition missing: %+v", cs.Diffs)
	}

	// the element was updated between h(24) and h(30): seen once, on the first claim after it
	cs = changeSet(findEvent(r, "claim:CP06"), "element")
	if diffAt(cs, "description") == nil || diffAt(cs, "recordhistory.lastupdated") == nil {
		t.Errorf("element change missing on CP06: %+v", cs)
	}
	if changeSet(findEvent(r, "claim:CE06"), "element") != nil {
		t.Errorf("CE06 has the same element copy as CP06")
	}
	if cs := changeSet(findEvent(r, "claim:CP01"), "element"); cs == nil || !cs.First {
		t.Errorf("the first claim should say there is nothing to compare the element with")
	}
	// the pcrs intent changed before h(54)
	if d := diffAt(changeSet(findEvent(r, "claim:CP10"), "intent"), "description"); d == nil {
		t.Errorf("intent change missing on CP10")
	}
	// outcomes
	if d := diffAt(changeSet(findEvent(r, "result:RP07"), "outcome"), "result"); d == nil || d.Old.Text != "0 Success" || d.New.Text != "9001 Fail" {
		t.Errorf("RP07 outcome change missing")
	}
	if changeSet(findEvent(r, "result:RP08"), "outcome") != nil {
		t.Errorf("RP08 outcome did not change")
	}
	if d := diffAt(changeSet(findEvent(r, "result:RE10"), "outcome"), "result"); d == nil || d.New.Text != "0 Success" {
		t.Errorf("RE10 outcome change back to success missing")
	}
	// the expected value was updated by jp update at h(50): first used at h(54)
	if d := diffAt(changeSet(findEvent(r, "result:RE10"), "expectedvalue"), "evs.secureboot"); d == nil || d.Old.Text != "1" || d.New.Text != "0" {
		t.Errorf("EV change missing on RE10")
	}
	if findEvent(r, "ev-updated:EVEFI") == nil {
		t.Errorf("EV update event missing")
	}
	// now: the element is as in its last copy, the EVs and intents too
	if e := findEvent(r, "element-current:E1"); e == nil || len(e.Changes) != 0 || !strings.Contains(e.Detail, "unchanged") {
		t.Errorf("element now: %+v", e)
	}
	if findEvent(r, "ev-current:EVEFI") != nil || findEvent(r, "intent-current:IPCR") != nil {
		t.Errorf("unexpected 'now' differences")
	}
	// series
	if len(r.Series) != 4 {
		t.Errorf("want 2 claim and 2 result series, got %+v", r.Series)
	}
	// links
	if e := findEvent(r, "claim:CP07"); e == nil || !hasRef(e, "result:RP07") || !hasRef(e, "session-opened:S07") {
		t.Errorf("CP07 refs: %+v", e.Refs)
	}
	// time order
	for i := 1; i < len(r.Events); i++ {
		if r.Events[i].T < r.Events[i-1].T {
			t.Fatalf("events out of order at %d", i)
		}
	}
}

func hasRef(e *Event, id string) bool {
	for _, r := range e.Refs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestRangeWithBaseline(t *testing.T) {
	f := story()
	now := time.Now()
	tr, err := parseRange(t0.Add(30*time.Hour).Format(time.RFC3339), t0.Add(42*time.Hour+time.Minute).Format(time.RFC3339), now)
	if err != nil {
		t.Fatal(err)
	}
	c, r := runStory(t, f, tr, now)
	if len(c.claims) != 6 {
		t.Fatalf("claims in range: %d, want 6", len(c.claims))
	}
	// one baseline claim per series and for the element: the two at h(24)
	if len(c.baseClaims) != 2 {
		t.Fatalf("baseline claims: %d, want 2", len(c.baseClaims))
	}
	if f.gets["/claim/CP01"] != 0 || f.gets["/claim/CP04"] != 0 {
		t.Errorf("claims older than the baseline were read")
	}
	if f.gets["/log/since"] != 1 || f.gets["/log"] != 0 {
		t.Errorf("the log should be read with /log/since, got %v", f.gets)
	}
	if findEvent(r, "claim:CP05") != nil {
		t.Errorf("a baseline claim is shown as an event")
	}
	cs := changeSet(findEvent(r, "claim:CP06"), "element")
	if cs == nil || cs.Against != "CE05" || cs.AgainstDoc != "claim:CE05" || r.Documents["claim:CE05"] == "" {
		t.Errorf("CP06 element change should be against baseline CE05: %+v", cs)
	}
	if changeSet(findEvent(r, "claim:CP07"), "body") == nil {
		t.Errorf("CP07 body change missing")
	}
	if findEvent(r, "element-current:E1") != nil {
		t.Errorf("a range in the past has no 'now' events")
	}
	if findEvent(r, "log:LE1") != nil || findEvent(r, "log:LCP06") == nil {
		t.Errorf("log entries not limited to the range")
	}
	for _, e := range r.Events {
		if !tr.contains(e.T*int64(time.Millisecond)) && !tr.contains(e.T*int64(time.Millisecond)+int64(time.Millisecond)-1) {
			t.Errorf("event outside the range: %s at %d", e.ID, e.T)
		}
	}
}

func TestEmptyRange(t *testing.T) {
	f := story()
	now := time.Now()
	tr, _ := parseRange("2020-01-01", "2020-01-02", now)
	c, r := runStory(t, f, tr, now)
	if len(c.claims) != 0 || len(c.baseClaims) != 0 || len(r.Events) != 0 {
		t.Errorf("want nothing, got %d claims, %d baselines, %d events", len(c.claims), len(c.baseClaims), len(r.Events))
	}
	// claims are newest first: everything after 2020 is read once and skipped, nothing more
	if f.gets["/claim/CP01"] != 1 {
		t.Errorf("CP01 read %d times", f.gets["/claim/CP01"])
	}
}

func TestElementMissingAndAPIKey(t *testing.T) {
	f := story()
	f.apiKey = "jane_" + strings.Repeat("A", 43)
	srv := httptest.NewServer(f)
	defer srv.Close()

	_, err := collect(newClient(srv.URL, ""), "E1", timeRange{All: true}, 10, time.Now(), discard{})
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("no key: want errUnauthorized, got %v", err)
	}
	_, err = collect(newClient(srv.URL, f.apiKey), "NOPE", timeRange{All: true}, 10, time.Now(), discard{})
	if err == nil || !strings.Contains(err.Error(), "NOPE") {
		t.Errorf("missing element: %v", err)
	}
	c, err := collect(newClient(srv.URL, f.apiKey), "E1", timeRange{All: true}, 10, time.Now(), discard{})
	if err != nil {
		t.Fatal(err)
	}
	// --max 10 cuts the lists short: warned
	if len(c.warnings) < 2 {
		t.Errorf("want warnings about --max, got %v", c.warnings)
	}
}

func TestRenderHTMLEscapes(t *testing.T) {
	f := story()
	f.claims["CP03"]["body"] = map[string]any{"evil": "</script><script>alert(1)</script>"}
	_, r := runStory(t, f, timeRange{All: true}, time.Now())
	page, err := renderHTML(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if strings.Contains(s, "alert(1)</script>") || strings.Count(s, "</script>") != 1 {
		t.Errorf("data can end the script element")
	}
	if strings.Contains(s, reportPlaceholder) {
		t.Errorf("placeholder left in the page")
	}
	if !strings.Contains(s, `"elementid":"E1"`) {
		t.Errorf("report data missing")
	}
	for _, ext := range []string{"http://", "https://", "src=\"", "@import"} {
		if strings.Contains(reportTemplate, ext) {
			t.Errorf("report.html must be self-contained, found %q", ext)
		}
	}
}

func TestNoDocuments(t *testing.T) {
	f := story()
	srv := httptest.NewServer(f)
	defer srv.Close()
	c, err := collect(newClient(srv.URL, ""), "E1", timeRange{All: true}, defaultMax, time.Now(), discard{})
	if err != nil {
		t.Fatal(err)
	}
	r := buildReport(c, timeRange{All: true}, srv.URL, "t", time.Now(), false)
	if len(r.Documents) != 0 {
		t.Errorf("documents included with --nodocuments")
	}
	if e := findEvent(r, "claim:CP07"); e == nil || e.Doc != "" || changeSet(e, "body") == nil {
		t.Errorf("events and differences must stay without documents")
	}
}

func TestCommandWritesOnce(t *testing.T) {
	t.Setenv("JANE_API_KEY", "")
	srv := httptest.NewServer(story())
	defer srv.Close()
	out := t.TempDir() + "/r.html"
	o := cmdline{elementID: "E1", server: srv.URL, out: out, max: defaultMax}
	tr, _ := parseRange("7d", "", time.Now())
	if err := forensicsCommand(o, tr, time.Now()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"elementid":"E1"`) {
		t.Fatalf("page not written: %v", err)
	}
	if err := forensicsCommand(o, tr, time.Now()); err == nil || !strings.Contains(err.Error(), "never overwrites") {
		t.Errorf("an existing file must not be overwritten: %v", err)
	}
}
