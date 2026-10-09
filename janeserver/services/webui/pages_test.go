package webui

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"strings"
	"testing"
	"time"

	"a10/structures"
)

// ---------------------------------------------------------------- fixtures

// richFixture extends the element-page fixture with the cross references the
// structure pages use: claim elements, result elements and expected values.
func richFixture(nSessions int) (structures.Element, []structures.Claim, []structures.Result, map[string]structures.Session, map[string]structures.ExpectedValue, structures.Intent) {
	e, cs, rs, sess := fixture(nSessions)
	intent := structures.Intent{ItemID: "in-quote", Name: "tpm2/quote", Function: "tpm2/quote",
		Description: "Obtain a TPM 2.0 quote over the selected PCRs", Parameters: map[string]interface{}{"pcrSelection": "sha256:0,1,2,3,4,5,6,7", "akhandle": "0x810100AA"}}
	evs := map[string]structures.ExpectedValue{}
	for i := range cs {
		cs[i].Header.Element = structures.Element{ItemID: e.ItemID, Name: e.Name}
		cs[i].Header.Endpoint = e.Endpoints["tarzan"]
		cs[i].Header.Timing.Received = cs[i].Header.Timing.Requested + structures.Timestamp(250*time.Millisecond)
		if cs[i].Header.Intent.Name == "tpm2/quote" {
			cs[i].Header.Intent = intent
		}
	}
	for i := range rs {
		rs[i].ElementID = e.ItemID
		evid := "ev-" + rs[i].RuleName
		rs[i].ExpectedValue = structures.ExpectedValue{ItemID: evid}
		rs[i].Parameters = map[string]interface{}{"pcrDigest": strings.Repeat("ab", 32)}
		if _, ok := evs[evid]; !ok {
			evs[evid] = structures.ExpectedValue{ItemID: evid, Name: rs[i].RuleName + " expected", Description: "Golden value for " + rs[i].RuleName,
				ElementID: e.ItemID, EndpointName: "tarzan", IntentID: intent.ItemID,
				EVS: map[string]interface{}{"attestedValue": strings.Repeat("0123456789abcdef", 6), "firmwareVersion": "2000.0.0"}}
		}
	}
	return e, cs, rs, sess, evs, intent
}

func testLookups(e structures.Element, i structures.Intent, evs map[string]structures.ExpectedValue, calls *int) *lookups {
	return &lookups{
		Element: func(id string) (structures.Element, bool) {
			*calls++
			return e, id == e.ItemID
		},
		Intent: func(id string) (structures.Intent, bool) {
			*calls++
			return i, id == i.ItemID
		},
		ExpectedValue: func(id string) (structures.ExpectedValue, bool) {
			*calls++
			ev, ok := evs[id]
			return ev, ok
		},
	}
}

func sessionsNewestFirst(m map[string]structures.Session) []structures.Session {
	var out []structures.Session
	for i := 200; i >= 0; i-- {
		if s, ok := m[fmt.Sprintf("sess-%02d", i)]; ok {
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------- pager

func TestNewPager(t *testing.T) {
	cases := []struct {
		param          string
		total          int64
		page, pages    int
		from, to, skip int64
		links          string
	}{
		{"", 0, 1, 1, 0, 0, 0, ""},
		{"", 50, 1, 1, 1, 50, 0, ""},
		{"2", 250, 2, 3, 101, 200, 100, "« 1 2 3 »"},
		{"x", 250, 1, 3, 1, 100, 0, "« 1 2 3 »"},
		{"99", 250, 3, 3, 201, 250, 200, "« 1 2 3 »"},
		{"-4", 250, 1, 3, 1, 100, 0, "« 1 2 3 »"},
		{"10", 2000, 10, 20, 901, 1000, 900, "« 1 … 8 9 10 11 12 … 20 »"},
		{"", -1, 1, 1, 0, 0, 0, ""},
	}
	for _, c := range cases {
		p := newPager(c.param, c.total, 100, "/x")
		var labels []string
		for _, l := range p.Links {
			labels = append(labels, l.Label)
		}
		if p.Page != c.page || p.Pages != c.pages || p.From != c.from || p.To != c.to || p.Skip != c.skip || strings.Join(labels, " ") != c.links {
			t.Errorf("newPager(%q,%d) = page %d/%d items %d-%d skip %d links %q", c.param, c.total, p.Page, p.Pages, p.From, p.To, p.Skip, strings.Join(labels, " "))
		}
	}
	p := newPager("1", 250, 100, "/sessions")
	if !p.Links[0].Disabled || p.Links[len(p.Links)-1].Href != "/sessions?page=2" || !p.Links[1].Active {
		t.Errorf("first page links wrong: %+v", p.Links)
	}
}

// ---------------------------------------------------------------- builders

func TestResultTypes(t *testing.T) {
	rs := []structures.Result{{Result: structures.Success}, {Result: structures.Success}, {Result: structures.Fail},
		{Result: structures.NoResult}, {Result: structures.ResultValue(4242)}}
	got := resultTypes(rs)
	var parts []string
	for _, g := range got {
		parts = append(parts, fmt.Sprintf("%s:%s:%d", g.Label, g.Class, g.Count))
	}
	want := "Pass:pass:2 Fail:fail:1 No result:other:1 Unknown:other:1"
	if strings.Join(parts, " ") != want {
		t.Fatalf("got %q want %q", strings.Join(parts, " "), want)
	}
	if got[0].Pct != 40 {
		t.Fatalf("pct %v", got[0].Pct)
	}
}

func TestBuildSessionsPage(t *testing.T) {
	e, cs, rs, sess, _, _ := richFixture(5)
	ss := sessionsNewestFirst(sess)
	page := buildSessionsPage(newPager("", int64(len(ss)), 100, "/sessions"), ss, cs, rs)
	if len(page.Rows) != 4 { // the fixture has no record for sess-03
		t.Fatalf("rows %d", len(page.Rows))
	}
	top := page.Rows[0]
	if top.S.ItemID != "sess-04" || !top.Open || top.Claims != 4 || top.Results.Total() != 6 {
		t.Fatalf("top row %+v", top)
	}
	if len(top.Elements) != 1 || top.Elements[0].Name != e.Name {
		t.Fatalf("elements %+v", top.Elements)
	}
	if page.Rows[1].Duration != "1.34s" || page.Rows[1].Open {
		t.Fatalf("closed session %+v", page.Rows[1])
	}
}

func TestBuildSessionPage(t *testing.T) {
	e, cs, rs, sess, evs, in := richFixture(5)
	var scs []structures.Claim
	var srs []structures.Result
	for _, c := range cs {
		if c.Header.Session.ItemID == "sess-02" {
			scs = append(scs, c)
		}
	}
	for _, r := range rs {
		if r.Session.ItemID == "sess-02" {
			srs = append(srs, r)
		}
	}
	// add a result for an element that made no claim in this session
	srs = append(srs, structures.Result{ItemID: "r-x", RuleName: "other_rule", ElementID: "e-other", Session: structures.Session{ItemID: "sess-02"}})
	calls := 0
	lk := testLookups(e, in, evs, &calls)
	lk.Element = func(id string) (structures.Element, bool) {
		calls++
		if id == "e-other" {
			return structures.Element{ItemID: id, Name: "zz-other-node"}, true
		}
		return e, true
	}
	page := buildSessionPage(sess["sess-02"], scs, srs, lk)
	if len(page.Elements) != 2 || page.Elements[0].Name != e.Name || page.Elements[1].Name != "zz-other-node" {
		t.Fatalf("elements %+v", page.Elements)
	}
	if calls != 1 {
		t.Fatalf("element lookups %d, want 1 (only for the element without claims)", calls)
	}
	if page.ClaimErrors != 1 || page.Elements[0].ClaimErrors != 1 || len(page.Elements[0].Claims) != 4 {
		t.Fatalf("claim errors %d / %d", page.ClaimErrors, page.Elements[0].ClaimErrors)
	}
	rules := page.Elements[0].Results
	for k := 1; k < len(rules); k++ {
		if rules[k-1].RuleName > rules[k].RuleName {
			t.Fatal("results not ordered by rule name")
		}
	}
	if page.ElementNames["e-other"] != "zz-other-node" || page.Open || page.Duration == "" {
		t.Fatalf("names/duration %+v %v %q", page.ElementNames, page.Open, page.Duration)
	}
}

func TestBuildResultsPageCachesLookups(t *testing.T) {
	e, _, rs, _, evs, in := richFixture(20)
	calls := 0
	page := buildResultsPage(newPager("", int64(len(rs)), 100, "/results"), rs, testLookups(e, in, evs, &calls))
	if len(page.Rows) != len(rs) {
		t.Fatal("rows")
	}
	// 6 distinct expected values + 1 element + 1 intent, however many rows
	if calls != len(evs)+2 {
		t.Fatalf("lookups %d, want %d", calls, len(evs)+2)
	}
	r := page.Rows[0]
	if r.Element.Name != e.Name || r.Intent.Name != in.Name || r.EV.Name == "" {
		t.Fatalf("resolved row %+v", r)
	}
	if page.Counts.Total() != len(rs) {
		t.Fatal("counts")
	}
}

func TestBuildResultPage(t *testing.T) {
	e, _, rs, _, evs, in := richFixture(3)
	calls := 0
	var siblings []structures.Result
	for _, r := range rs {
		if r.ClaimID == rs[0].ClaimID {
			siblings = append(siblings, r)
		}
	}
	page := buildResultPage(rs[0], siblings, testLookups(e, in, evs, &calls))
	if page.EV.Name == "" || len(page.EV.EVS) == 0 || page.R.ItemID != rs[0].ItemID {
		t.Fatalf("ev %+v", page.EV)
	}
	if len(page.Siblings) != len(siblings)-1 {
		t.Fatal("siblings should exclude the result itself")
	}
	for _, s := range page.Siblings {
		if s.ItemID == rs[0].ItemID {
			t.Fatal("result listed as its own sibling")
		}
	}
}

func TestBuildClaimPage(t *testing.T) {
	c := structures.Claim{ItemID: "c1", BodyType: structures.CLAIMERROR, Body: map[string]interface{}{"ERROR": map[string]interface{}{"msg": "endpoint unreachable"}}}
	c.Header.Timing.Requested, c.Header.Timing.Received = 1, structures.Timestamp(2*time.Second)+1
	page := buildClaimPage(c, nil)
	if !page.IsError || page.Latency != "2s" || !strings.Contains(page.RawJSON, `"msg": "endpoint unreachable"`) {
		t.Fatalf("%+v", page)
	}
}

func TestBuildIntentsAndIntentPage(t *testing.T) {
	e, cs, _, _, evs, in := richFixture(3)
	var evList []structures.ExpectedValue
	for _, ev := range evs {
		evList = append(evList, ev)
	}
	other := structures.Intent{ItemID: "in-a", Name: "a/first", Function: "sys/info"}
	cards := buildIntentsPage([]structures.Intent{in, other}, evList, func(id string) int64 { return map[string]int64{"in-quote": 3}[id] })
	if cards[0].I.Name != "a/first" || cards[1].EVs != len(evs) || cards[1].Claims != 3 || cards[0].EVs != 0 {
		t.Fatalf("cards %+v", cards)
	}
	if c := buildIntentsPage([]structures.Intent{in}, nil, nil); c[0].Claims != -1 {
		t.Fatal("unknown claim count should be -1")
	}
	calls := 0
	page := buildIntentPage(in, evList, cs[:2], 3, testLookups(e, in, evs, &calls))
	if len(page.EVs) != len(evs) || page.EVs[0].Element.Name != e.Name || page.ClaimTotal != 3 {
		t.Fatalf("intent page %+v", page)
	}
}

func TestBuildEVPage(t *testing.T) {
	_, _, rs, _, evs, in := richFixture(4)
	page := buildEVPage(evs["ev-ima_policy"], structures.Element{}, in, rs[:5], 42)
	if len(page.Strip) != 5 || page.Strip[0].ItemID != rs[4].ItemID || page.Strip[4].ItemID != rs[0].ItemID {
		t.Fatal("strip should be oldest first")
	}
	if page.ResultTotal != 42 || page.Counts.Total() != 5 {
		t.Fatal("totals")
	}
}

// ---------------------------------------------------------------- rendering

// TestRenderStructurePages parses every template exactly as the server does
// and renders each of the ten structure pages with data and empty/not-found
// data. Set RENDER_DIR to keep the HTML for inspection.
func TestRenderStructurePages(t *testing.T) {
	funcs := templateFunctions()
	funcs["opaqueObject"] = func(v string) template.HTML { return template.HTML(template.HTMLEscapeString(v)) }
	funcs["opaqueObjectInt64"] = func(v int64) template.HTML { return template.HTML(fmt.Sprint(v)) }
	tmpls := parseTemplates(funcs)

	e, cs, rs, sess, evs, in := richFixture(150)
	calls := 0
	lk := func() *lookups { return testLookups(e, in, evs, &calls) }
	ss := sessionsNewestFirst(sess)

	// claim bodies of each type the claim page knows about
	asciilog := "10 " + strings.Repeat("a", 40) + " ima-ng sha256:" + strings.Repeat("b", 64) + " /usr/bin/bash\n" +
		"10 " + strings.Repeat("c", 40) + " ima-ng sha256:" + strings.Repeat("d", 64) + " /usr/lib/systemd/systemd\n"
	quote := cs[0]
	for _, c := range cs {
		if c.BodyType == "tpm2/quote" {
			quote = c
			break
		}
	}
	quote.Body = map[string]interface{}{
		"quote": map[string]interface{}{
			"attested":        map[string]interface{}{"pcrdigest": strings.Repeat("9f", 32), "pcrselect": "sha256:0,1,2,3,4,5,6,7"},
			"firmwareVersion": "2000.0.0", "magic": "ff544347", "type": "8018", "qualifiedsigner": strings.Repeat("00", 34),
			"clockinfo": map[string]interface{}{"clock": 123456, "resetcount": 3, "restartcount": 0, "safe": true},
		},
		"signature": map[string]interface{}{"SigAlg": 20, "Signature": strings.Repeat("5a", 256)},
	}
	ima := cs[1]
	ima.BodyType = "ima/asciilog"
	ima.Body = map[string]interface{}{"encoded": "base64", "encodedlength": 400, "unencodedlength": 300, "asciilog": base64.StdEncoding.EncodeToString([]byte(asciilog))}
	errClaim := cs[2]
	errClaim.BodyType = structures.CLAIMERROR
	errClaim.Body = map[string]interface{}{"ERROR": map[string]interface{}{"APICALL": "http://10.0.0.21:8530/tpm2/quote", "error": "connection refused"}}
	pcrs := cs[3]
	pcrs.BodyType = "tpm2/pcrs"
	pcrs.Body = map[string]interface{}{"sha256": map[string]interface{}{"0": strings.Repeat("a1", 32), "1": strings.Repeat("b2", 32)}}
	unknown := cs[3]
	unknown.BodyType = "custom/thing"
	unknown.Body = map[string]interface{}{"field": "value"}

	var claimResults []structures.Result
	for _, r := range rs {
		if r.Session.ItemID == quote.Header.Session.ItemID {
			claimResults = append(claimResults, r)
		}
	}
	var s2cs []structures.Claim
	var s2rs []structures.Result
	for _, c := range cs {
		if c.Header.Session.ItemID == "sess-149" {
			s2cs = append(s2cs, c)
		}
	}
	for _, r := range rs {
		if r.Session.ItemID == "sess-149" {
			s2rs = append(s2rs, r)
		}
	}
	var evList []structures.ExpectedValue
	for _, ev := range evs {
		evList = append(evList, ev)
	}
	var evCards []evstruct
	for _, ev := range evList {
		evCards = append(evCards, evstruct{ev, e, in})
	}
	var evResults []structures.Result
	for _, r := range rs {
		if r.ExpectedValue.ItemID == "ev-ima_policy" && len(evResults) < evResultsShown {
			evResults = append(evResults, r)
		}
	}
	pg := func(param string, total int64, path string) pager { return newPager(param, total, listPageSize, path) }

	pages := []struct {
		name, tmpl string
		data       interface{}
		want       []string
	}{
		{"sessions", "sessions.html", buildSessionsPage(pg("2", int64(len(ss)), "/sessions"), ss[100:], cs, rs), []string{"Showing 101–149 of 149", `href="/sessions?page=1"`}},
		{"sessions-empty", "sessions.html", buildSessionsPage(pg("", 0, "/sessions"), nil, nil, nil), []string{"No sessions yet"}},
		{"session", "session.html", buildSessionPage(sess["sess-149"], s2cs, s2rs, lk()), []string{"Result types", "Still open", e.Name}},
		{"session-missing", "session.html", buildSessionPage(structures.Session{}, nil, nil, lk()), []string{"Session not found"}},
		{"claims", "claims.html", buildClaimsPage(pg("1", int64(len(cs)), "/claims"), cs[:100]), []string{"Showing 1–100 of 600", "errors on this page"}},
		{"claim-quote", "claim.html", buildClaimPage(quote, claimResults), []string{"PCR Digest", "Results from this claim", "ff544347"}},
		{"claim-ima", "claim.html", buildClaimPage(ima, nil), []string{"IMA ASCII log", "/usr/lib/systemd/systemd"}},
		{"claim-error", "claim.html", buildClaimPage(errClaim, nil), []string{"connection refused", "Error"}},
		{"claim-pcrs", "claim.html", buildClaimPage(pcrs, nil), []string{strings.Repeat("a1", 32)}},
		{"claim-unknown", "claim.html", buildClaimPage(unknown, nil), []string{"field", "custom/thing"}},
		{"claim-missing", "claim.html", buildClaimPage(structures.Claim{}, nil), []string{"Claim not found"}},
		{"results", "results.html", buildResultsPage(pg("1", int64(len(rs)), "/results"), rs[:100], lk()), []string{"This page:", "ima_policy expected", "via"}},
		{"result", "result.html", buildResultPage(rs[0], claimResults, lk()), []string{"Same claim", "attestedValue", "pcrDigest"}},
		{"result-missing", "result.html", buildResultPage(structures.Result{}, nil, lk()), []string{"Result not found"}},
		{"intents", "intents.html", buildIntentsPage([]structures.Intent{in, {ItemID: "in-b", Name: "sys/info", Function: "sys/info"}}, evList, func(string) int64 { return 7 }), []string{"7 claims", "6 expected values"}},
		{"intents-empty", "intents.html", buildIntentsPage(nil, nil, nil), []string{"No intents defined yet"}},
		{"intent", "intent.html", buildIntentPage(in, evList, cs[:100], 150, lk()), []string{"Showing the latest 100 of 150 claims", "pcrSelection"}},
		{"intent-missing", "intent.html", buildIntentPage(structures.Intent{}, nil, nil, 0, lk()), []string{"Intent not found"}},
		{"evs", "evs.html", evCards, []string{"2 values", "tpm2/quote"}},
		{"evs-empty", "evs.html", []evstruct{}, []string{"No expected values defined yet"}},
		{"ev", "ev.html", buildEVPage(evs["ev-ima_policy"], e, in, evResults, 150), []string{"Showing the latest 100 of 150 results", "History"}},
		{"ev-missing", "ev.html", buildEVPage(structures.ExpectedValue{}, structures.Element{}, structures.Intent{}, nil, 0), []string{"Expected value not found"}},
	}
	dir := os.Getenv("RENDER_DIR")
	for _, p := range pages {
		var b strings.Builder
		if err := tmpls[p.tmpl].ExecuteTemplate(&b, "base.html", p.data); err != nil {
			t.Errorf("%s: %v", p.name, err)
			continue
		}
		out := b.String()
		if strings.Contains(out, "ZgotmplZ") {
			t.Errorf("%s: a value was rejected by html/template (ZgotmplZ)", p.name)
		}
		for _, w := range p.want {
			if !strings.Contains(out, template.HTMLEscapeString(w)) && !strings.Contains(out, w) {
				t.Errorf("%s: missing %q", p.name, w)
			}
		}
		if dir != "" {
			os.WriteFile(dir+"/"+p.name+".html", []byte(out), 0o644)
		}
	}
}
