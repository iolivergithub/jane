package webui

// View models for the sessions, session, claims, claim, results, result,
// intents, intent, expected values and expected value pages. Everything here
// is a pure function of data already fetched; database lookups are passed in
// so the builders can be tested without a database.
//
// Counts are reported per category (pass / fail / other, collected / error).
// Nothing here combines results into an overall verdict: that is not Jane's job.

import (
	"encoding/json"
	"fmt"
	"sort"

	"a10/structures"
)

// ---------------------------------------------------------------- shared

// resultCounts counts results by class.
type resultCounts struct {
	Pass, Fail, Other          int
	PassPct, FailPct, OtherPct float64
}

func (rc *resultCounts) add(r structures.Result) {
	switch resultOutcome(r.Result) {
	case outcomePass:
		rc.Pass++
	case outcomeFail:
		rc.Fail++
	default:
		rc.Other++
	}
}

func (rc *resultCounts) finish() {
	t := rc.Pass + rc.Fail + rc.Other
	rc.PassPct, rc.FailPct, rc.OtherPct = pct(rc.Pass, t), pct(rc.Fail, t), pct(rc.Other, t)
}

func (rc resultCounts) Total() int { return rc.Pass + rc.Fail + rc.Other }

func countResults(rs []structures.Result) resultCounts {
	var rc resultCounts
	for _, r := range rs {
		rc.add(r)
	}
	rc.finish()
	return rc
}

// resultType is one result value and how many results had it.
type resultType struct {
	Value structures.ResultValue
	Label string
	Class string
	Count int
	Pct   float64
}

// resultValueOrder lists every result value in display order.
var resultValueOrder = []structures.ResultValue{
	structures.Success, structures.Fail,
	structures.VerifyCallFailure, structures.VerifyClaimErrorAttempt, structures.NoResult,
	structures.MissingExpectedValue, structures.RuleCallFailure, structures.UnsetResultValue,
}

// resultTypes counts results by exact value, in display order, leaving out
// values that do not occur; unknown values are grouped at the end.
func resultTypes(rs []structures.Result) []resultType {
	counts := map[structures.ResultValue]int{}
	for _, r := range rs {
		counts[r.Result]++
	}
	var out []resultType
	known := map[structures.ResultValue]bool{}
	for _, v := range resultValueOrder {
		known[v] = true
		if counts[v] > 0 {
			out = append(out, resultType{Value: v, Label: ResultLabel(v), Class: ResultClass(v), Count: counts[v]})
		}
	}
	unknown := 0
	for v, n := range counts {
		if !known[v] {
			unknown += n
		}
	}
	if unknown > 0 {
		out = append(out, resultType{Value: -1, Label: "Unknown", Class: outcomeOther, Count: unknown})
	}
	for i := range out {
		out[i].Pct = pct(out[i].Count, len(rs))
	}
	return out
}

func countClaimErrors(cs []structures.Claim) int {
	n := 0
	for _, c := range cs {
		if claimOutcome(c) == outcomeClaimErr {
			n++
		}
	}
	return n
}

// ref is a named link to another item.
type ref struct {
	ID, Name string
}

// lookups fetches related items; any of them may be nil. Results are cached
// so a page asks for each item at most once.
type lookups struct {
	Element       func(id string) (structures.Element, bool)
	Intent        func(id string) (structures.Intent, bool)
	ExpectedValue func(id string) (structures.ExpectedValue, bool)

	elements map[string]structures.Element
	intents  map[string]structures.Intent
	evs      map[string]structures.ExpectedValue
}

func (l *lookups) element(id string) structures.Element {
	if id == "" || l.Element == nil {
		return structures.Element{}
	}
	if l.elements == nil {
		l.elements = map[string]structures.Element{}
	}
	if e, ok := l.elements[id]; ok {
		return e
	}
	e, _ := l.Element(id)
	l.elements[id] = e
	return e
}

func (l *lookups) intent(id string) structures.Intent {
	if id == "" || l.Intent == nil {
		return structures.Intent{}
	}
	if l.intents == nil {
		l.intents = map[string]structures.Intent{}
	}
	if i, ok := l.intents[id]; ok {
		return i
	}
	i, _ := l.Intent(id)
	l.intents[id] = i
	return i
}

func (l *lookups) expectedValue(id string) structures.ExpectedValue {
	if id == "" || l.ExpectedValue == nil {
		return structures.ExpectedValue{}
	}
	if l.evs == nil {
		l.evs = map[string]structures.ExpectedValue{}
	}
	if ev, ok := l.evs[id]; ok {
		return ev
	}
	ev, _ := l.ExpectedValue(id)
	l.evs[id] = ev
	return ev
}

// ---------------------------------------------------------------- sessions

type sessionRow struct {
	S           structures.Session
	Duration    string
	Open        bool
	Results     resultCounts
	Claims      int
	ClaimErrors int
	Elements    []ref
}

type sessionsPage struct {
	Pager pager
	Rows  []sessionRow
}

// buildSessionsPage pairs each session with the claims and results recorded for it.
func buildSessionsPage(p pager, ss []structures.Session, cs []structures.Claim, rs []structures.Result) sessionsPage {
	page := sessionsPage{Pager: p}
	claims := map[string][]structures.Claim{}
	for _, c := range cs {
		claims[c.Header.Session.ItemID] = append(claims[c.Header.Session.ItemID], c)
	}
	results := map[string][]structures.Result{}
	for _, r := range rs {
		results[r.Session.ItemID] = append(results[r.Session.ItemID], r)
	}
	for _, s := range ss {
		row := sessionRow{S: s, Duration: formatDuration(s.Timing.Opened, s.Timing.Closed), Open: s.Timing.Closed == 0}
		row.Results = countResults(results[s.ItemID])
		row.Claims = len(claims[s.ItemID])
		row.ClaimErrors = countClaimErrors(claims[s.ItemID])
		seen := map[string]bool{}
		for _, c := range claims[s.ItemID] {
			e := c.Header.Element
			if e.ItemID != "" && !seen[e.ItemID] {
				seen[e.ItemID] = true
				row.Elements = append(row.Elements, ref{e.ItemID, e.Name})
			}
		}
		sort.Slice(row.Elements, func(a, b int) bool { return row.Elements[a].Name < row.Elements[b].Name })
		page.Rows = append(page.Rows, row)
	}
	return page
}

// ---------------------------------------------------------------- session

// sessionElement is one element attested in a session.
type sessionElement struct {
	ID, Name    string
	E           structures.Element // current record, or the copy in a claim header if it no longer exists
	Claims      []structures.Claim
	Results     []structures.Result // by rule name
	Counts      resultCounts
	ClaimErrors int
}

type sessionPage struct {
	S            structures.Session
	Duration     string
	Open         bool
	Claims       []structures.Claim  // newest first
	Results      []structures.Result // newest first
	Counts       resultCounts
	Types        []resultType
	ClaimErrors  int
	Elements     []sessionElement
	ElementNames map[string]string // element ID -> name
}

func buildSessionPage(s structures.Session, cs []structures.Claim, rs []structures.Result, lk *lookups) sessionPage {
	page := sessionPage{S: s, Duration: formatDuration(s.Timing.Opened, s.Timing.Closed), Open: s.Timing.Closed == 0,
		Claims: cs, Results: rs, Counts: countResults(rs), Types: resultTypes(rs), ClaimErrors: countClaimErrors(cs)}

	byID := map[string]*sessionElement{}
	var order []string
	get := func(id, name string) *sessionElement {
		se, ok := byID[id]
		if !ok {
			se = &sessionElement{ID: id, Name: name}
			byID[id] = se
			order = append(order, id)
		}
		if se.Name == "" {
			se.Name = name
		}
		return se
	}
	for _, c := range cs {
		se := get(c.Header.Element.ItemID, c.Header.Element.Name)
		se.Claims = append(se.Claims, c)
	}
	for _, r := range rs {
		se := get(r.ElementID, "")
		se.Results = append(se.Results, r)
	}
	for _, id := range order {
		se := byID[id]
		if lk != nil {
			if cur := lk.element(id); cur.ItemID != "" {
				se.E = cur
			}
		}
		if se.E.ItemID == "" && len(se.Claims) > 0 {
			se.E = se.Claims[0].Header.Element
		}
		if se.E.Name != "" {
			se.Name = se.E.Name
		}
		if se.Name == "" {
			se.Name = id
		}
		sort.SliceStable(se.Results, func(a, b int) bool { return se.Results[a].RuleName < se.Results[b].RuleName })
		sort.SliceStable(se.Claims, func(a, b int) bool { return claimLabel(se.Claims[a]) < claimLabel(se.Claims[b]) })
		se.Counts = countResults(se.Results)
		se.ClaimErrors = countClaimErrors(se.Claims)
		page.Elements = append(page.Elements, *se)
	}
	sort.SliceStable(page.Elements, func(a, b int) bool { return page.Elements[a].Name < page.Elements[b].Name })
	page.ElementNames = map[string]string{}
	for _, se := range page.Elements {
		page.ElementNames[se.ID] = se.Name
	}
	return page
}

// ---------------------------------------------------------------- claims

type claimRow struct {
	C       structures.Claim
	Latency string
}

type claimsPage struct {
	Pager       pager
	Rows        []claimRow
	ClaimErrors int // on this page
}

func buildClaimsPage(p pager, cs []structures.Claim) claimsPage {
	page := claimsPage{Pager: p, ClaimErrors: countClaimErrors(cs)}
	for _, c := range cs {
		page.Rows = append(page.Rows, claimRow{C: c, Latency: formatDuration(c.Header.Timing.Requested, c.Header.Timing.Received)})
	}
	return page
}

// ---------------------------------------------------------------- claim

// claimPage embeds the claim so the claim body partials keep working unchanged.
type claimPage struct {
	structures.Claim
	Latency string
	IsError bool
	Results []structures.Result // results produced by verifying this claim
	Counts  resultCounts
	RawJSON string
}

func buildClaimPage(c structures.Claim, rs []structures.Result) claimPage {
	raw, err := json.MarshalIndent(c.Body, "", "  ")
	rawText := string(raw)
	if err != nil {
		rawText = fmt.Sprintf("%v", c.Body)
	}
	return claimPage{Claim: c, Latency: formatDuration(c.Header.Timing.Requested, c.Header.Timing.Received),
		IsError: claimOutcome(c) == outcomeClaimErr, Results: rs, Counts: countResults(rs), RawJSON: rawText}
}

// ---------------------------------------------------------------- results

type resultRow struct {
	R       structures.Result
	Element ref
	Intent  ref
	EV      ref
}

type resultsPage struct {
	Pager  pager
	Rows   []resultRow
	Counts resultCounts // on this page
}

func resolveResult(r structures.Result, lk *lookups) resultRow {
	row := resultRow{R: r}
	evid := r.ExpectedValue.ItemID
	ev := lk.expectedValue(evid)
	if ev.ItemID == "" {
		ev = r.ExpectedValue // the copy stored with the result
	}
	row.EV = ref{ev.ItemID, ev.Name}
	eid := r.ElementID
	if eid == "" {
		eid = ev.ElementID
	}
	if eid != "" {
		row.Element = ref{eid, lk.element(eid).Name}
	}
	if ev.IntentID != "" {
		row.Intent = ref{ev.IntentID, lk.intent(ev.IntentID).Name}
	}
	return row
}

func buildResultsPage(p pager, rs []structures.Result, lk *lookups) resultsPage {
	page := resultsPage{Pager: p, Counts: countResults(rs)}
	for _, r := range rs {
		page.Rows = append(page.Rows, resolveResult(r, lk))
	}
	return page
}

// ---------------------------------------------------------------- result

type resultPage struct {
	resultRow
	EV       structures.ExpectedValue
	Siblings []structures.Result // other results from the same claim
}

func buildResultPage(r structures.Result, siblings []structures.Result, lk *lookups) resultPage {
	page := resultPage{resultRow: resolveResult(r, lk)}
	page.EV = lk.expectedValue(r.ExpectedValue.ItemID)
	if page.EV.ItemID == "" {
		page.EV = r.ExpectedValue
	}
	for _, s := range siblings {
		if s.ItemID != r.ItemID {
			page.Siblings = append(page.Siblings, s)
		}
	}
	return page
}

// ---------------------------------------------------------------- intents

type intentCard struct {
	I      structures.Intent
	EVs    int
	Claims int64 // -1 when unknown
}

func buildIntentsPage(is []structures.Intent, evs []structures.ExpectedValue, claimCount func(id string) int64) []intentCard {
	perIntent := map[string]int{}
	for _, ev := range evs {
		perIntent[ev.IntentID]++
	}
	out := make([]intentCard, 0, len(is))
	for _, i := range is {
		n := int64(-1)
		if claimCount != nil {
			n = claimCount(i.ItemID)
		}
		out = append(out, intentCard{I: i, EVs: perIntent[i.ItemID], Claims: n})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].I.Name < out[b].I.Name })
	return out
}

// ---------------------------------------------------------------- intent

type evRef struct {
	EV      structures.ExpectedValue
	Element ref
}

type intentPage struct {
	I           structures.Intent
	EVs         []evRef
	Claims      []structures.Claim // most recent, capped
	ClaimTotal  int64              // -1 when unknown
	ClaimErrors int                // among Claims
}

func buildIntentPage(i structures.Intent, evs []structures.ExpectedValue, cs []structures.Claim, total int64, lk *lookups) intentPage {
	page := intentPage{I: i, Claims: cs, ClaimTotal: total, ClaimErrors: countClaimErrors(cs)}
	for _, ev := range evs {
		if ev.IntentID == i.ItemID {
			page.EVs = append(page.EVs, evRef{EV: ev, Element: ref{ev.ElementID, lk.element(ev.ElementID).Name}})
		}
	}
	sort.SliceStable(page.EVs, func(a, b int) bool {
		if page.EVs[a].Element.Name != page.EVs[b].Element.Name {
			return page.EVs[a].Element.Name < page.EVs[b].Element.Name
		}
		return page.EVs[a].EV.Name < page.EVs[b].EV.Name
	})
	return page
}

// ---------------------------------------------------------------- expected value

type evPage struct {
	EV          structures.ExpectedValue
	E           structures.Element
	I           structures.Intent
	Results     []structures.Result // newest first, capped
	Strip       []structures.Result // the same results oldest first, for the history strip
	ResultTotal int64               // -1 when unknown
	Counts      resultCounts
}

func buildEVPage(ev structures.ExpectedValue, e structures.Element, i structures.Intent, rs []structures.Result, total int64) evPage {
	page := evPage{EV: ev, E: e, I: i, Results: rs, ResultTotal: total, Counts: countResults(rs)}
	for k := len(rs) - 1; k >= 0; k-- {
		page.Strip = append(page.Strip, rs[k])
	}
	return page
}
