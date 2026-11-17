package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event is one entry in the timeline.
type Event struct {
	ID         string      `json:"id"`   // type:itemid, unique in the report
	T          int64       `json:"t"`    // milliseconds since 1970 (UTC)
	NS         string      `json:"ns"`   // the Jane timestamp, nanoseconds, exact
	Type       string      `json:"type"` // element intent ev claim result session log
	Kind       string      `json:"kind"` // created updated current claim result opened closed log
	Title      string      `json:"title"`
	Detail     string      `json:"detail,omitempty"`
	ItemID     string      `json:"itemid,omitempty"`
	Doc        string      `json:"doc,omitempty"` // key into the report's documents
	Intent     string      `json:"intent,omitempty"`
	IntentID   string      `json:"intentid,omitempty"`
	Endpoint   string      `json:"endpoint,omitempty"`
	Rule       string      `json:"rule,omitempty"`
	RuleDesc   string      `json:"ruledesc,omitempty"`
	Function   string      `json:"function,omitempty"` // claims: the intent's function, e.g. tpm2/quote
	DecodedDoc string      `json:"decodeddoc,omitempty"`
	Prev       *Prev       `json:"prev,omitempty"` // the previous claim or result of the same kind
	EV         string      `json:"ev,omitempty"`   // results: the expected value's itemid
	Series     string      `json:"series,omitempty"`
	Result     *int64      `json:"result,omitempty"`
	ResultName string      `json:"resultname,omitempty"`
	Session    string      `json:"session,omitempty"`
	Refs       []Ref       `json:"refs,omitempty"`
	Changes    []ChangeSet `json:"changes,omitempty"`
}

// Prev is the item an event is compared with: the previous claim of the same
// intent on the same endpoint, or the previous result of the same rule.
type Prev struct {
	ID    string `json:"id"` // event ID or document key
	T     int64  `json:"t"`
	Label string `json:"label"`
}

// Ref links an event to another event or document.
type Ref struct {
	Label string `json:"label"`
	ID    string `json:"id"` // an event ID, or a document key if there is no such event
}

// ChangeSet is what changed in one part of an event against the one before.
type ChangeSet struct {
	What       string        `json:"what"` // element intent body endpoint expectedvalue outcome
	Label      string        `json:"label"`
	Against    string        `json:"against,omitempty"` // itemid compared with
	AgainstDoc string        `json:"againstdoc,omitempty"`
	AgainstT   int64         `json:"againstt,omitempty"`
	First      bool          `json:"first,omitempty"` // nothing earlier to compare with
	Diffs      []Diff        `json:"diffs,omitempty"`
	Summary    []SummaryLine `json:"summary,omitempty"`
}

// Series is one intent on one endpoint (claims), or that and a rule (results).
type Series struct {
	Key      string `json:"key"`
	Type     string `json:"type"` // claim or result
	Intent   string `json:"intent"`
	IntentID string `json:"intentid"`
	Endpoint string `json:"endpoint"`
	Rule     string `json:"rule,omitempty"`
}

// Report is everything the HTML page shows.
type Report struct {
	Tool        string            `json:"tool"`
	Generated   string            `json:"generated"`
	Server      string            `json:"server"`
	ElementID   string            `json:"elementid"`
	ElementName string            `json:"elementname"`
	Range       string            `json:"range"`
	FromMS      int64             `json:"from,omitempty"`
	UntilMS     int64             `json:"until,omitempty"`
	Counts      map[string]int    `json:"counts"`
	Warnings    []string          `json:"warnings"`
	Series      []Series          `json:"series"`
	Events      []Event           `json:"events"`
	Documents   map[string]string `json:"documents"`
}

var resultNames = map[int64]string{
	0:    "Success",
	9001: "Fail",
	9010: "VerifyCallFailure",
	9098: "VerifyClaimErrorAttempt",
	9099: "NoResult",
	9997: "MissingExpectedValue",
	9998: "RuleCallFailure",
	9999: "UnsetResultValue",
}

func resultName(code int64) string {
	if n, ok := resultNames[code]; ok {
		return fmt.Sprintf("%d %s", code, n)
	}
	return fmt.Sprintf("%d unknown result", code)
}

func ms(ns int64) int64 { return ns / int64(time.Millisecond) }

// builder turns a collection into a report.
type builder struct {
	c         *collection
	r         *Report
	withDocs  bool
	events    map[string]bool
	intentNms map[string]string
	views     map[*doc]cachedView
}

func buildReport(c *collection, tr timeRange, server, tool string, now time.Time, withDocs bool) *Report {
	r := &Report{
		Tool:        tool,
		Generated:   now.UTC().Format(time.RFC3339),
		Server:      server,
		ElementID:   c.element.str("itemid"),
		ElementName: c.element.str("name"),
		Range:       tr.String(),
		Warnings:    append([]string{}, c.warnings...),
		Documents:   map[string]string{},
		Counts:      map[string]int{},
	}
	if !tr.All {
		r.FromMS, r.UntilMS = tr.From.UnixMilli(), tr.Until.UnixMilli()
	}
	b := &builder{c: c, r: r, withDocs: withDocs, events: map[string]bool{}, intentNms: map[string]string{}, views: map[*doc]cachedView{}}
	for id, d := range c.intents {
		b.intentNms[id] = d.str("name")
	}
	for _, d := range append(append([]*doc{}, c.baseClaims...), c.claims...) {
		if id := d.str("header.intent.itemid"); id != "" && b.intentNms[id] == "" {
			b.intentNms[id] = d.str("header.intent.name")
		}
	}

	b.claims(tr, now)
	b.results(tr, now)
	b.element(tr, now)
	b.expectedValues(tr)
	b.sessions(tr)
	b.logEntries()
	b.links()

	sort.SliceStable(r.Events, func(i, j int) bool {
		a, bb := r.Events[i], r.Events[j]
		na, _ := strconv.ParseInt(a.NS, 10, 64)
		nb, _ := strconv.ParseInt(bb.NS, 10, 64)
		if na != nb {
			return na < nb
		}
		return a.ID < bb.ID
	})

	r.Counts["claims"] = len(c.claims)
	r.Counts["baselineclaims"] = len(c.baseClaims)
	r.Counts["results"] = len(c.results)
	r.Counts["baselineresults"] = len(c.baseResults)
	r.Counts["expectedvalues"] = len(c.evs)
	r.Counts["intents"] = len(c.intents)
	r.Counts["sessions"] = len(c.sessions)
	r.Counts["log"] = len(c.logs)
	r.Counts["events"] = len(r.Events)
	changed := 0
	for _, e := range r.Events {
		if hasChange(e) {
			changed++
		}
	}
	r.Counts["eventswithchanges"] = changed
	return r
}

func (b *builder) addDoc(key string, d *doc) string {
	if d == nil {
		return ""
	}
	if b.withDocs {
		b.r.Documents[key] = pretty(d.raw)
		return key
	}
	return ""
}

func (b *builder) add(e Event) {
	if e.Type != "claim" {
		b.finish(e.Changes, sumCtx{})
	}
	b.events[e.ID] = true
	b.r.Events = append(b.r.Events, e)
}

func (b *builder) intentName(id string) string {
	if n := b.intentNms[id]; n != "" {
		return n
	}
	return id
}

// claims: each claim in the range, compared with the claim before it (any
// intent) for the element copy, the claim before it with the same intent for
// the intent copy, and the claim before it in its series for the body and
// endpoint. Baseline claims are compared with but not shown.
func (b *builder) claims(tr timeRange, now time.Time) {
	c := b.c
	var lastElement *doc
	lastIntent := map[string]*doc{}
	lastSeries := map[string]*doc{}
	seenSeries := map[string]bool{}

	all := byTime(append(append([]*doc{}, c.baseClaims...), c.claims...), claimTime)
	inRange := map[*doc]bool{}
	for _, d := range c.claims {
		inRange[d] = true
	}
	for _, d := range all {
		id := d.str("itemid")
		pid := d.str("header.intent.itemid")
		key := claimSeries(d)
		if !inRange[d] {
			b.addDoc("claim:"+id, d) // baselines are compared with, so keep them
			lastElement, lastIntent[pid], lastSeries[key] = d, d, d
			continue
		}
		e := Event{
			ID:       "claim:" + id,
			T:        ms(claimTime(d)),
			NS:       strconv.FormatInt(claimTime(d), 10),
			Type:     "claim",
			Kind:     "claim",
			ItemID:   id,
			Doc:      b.addDoc("claim:"+id, d),
			Intent:   b.intentName(pid),
			IntentID: pid,
			Endpoint: d.str("header.endpointname"),
			Series:   key,
			Session:  d.str("header.session.itemid"),
		}
		e.Function = d.str("header.intent.function")
		e.Title = fmt.Sprintf("Claim: %s on %s", e.Intent, e.Endpoint)
		if prev := lastSeries[key]; prev != nil {
			e.Prev = &Prev{ID: "claim:" + prev.str("itemid"), T: ms(claimTime(prev)),
				Label: fmt.Sprintf("claim of %s on %s", e.Intent, e.Endpoint)}
		}
		if bt := d.str("bodytype"); bt != "" {
			e.Detail = "body type " + bt
		}
		if received := d.num("header.timing.received"); received > 0 {
			e.Detail = join2(e.Detail, fmt.Sprintf("answered in %s", time.Duration(received-claimTime(d)).Round(time.Millisecond)))
		}
		e.Changes = appendChange(e.Changes, "element", "Element copy (vs previous claim)", "claim", lastElement, d, "header.element", true)
		e.Changes = appendChange(e.Changes, "intent", "Intent copy (vs previous claim of this intent)", "claim", lastIntent[pid], d, "header.intent", true)
		e.Changes = appendChange(e.Changes, "endpoint", "Endpoint (vs previous claim in this series)", "claim", lastSeries[key], d, "header.endpoint", false)
		e.Changes = appendChange(e.Changes, "body", "Claim body (vs previous claim in this series)", "claim", lastSeries[key], d, "body", true)
		view, title := b.view(d)
		if view != nil {
			if b.withDocs {
				e.DecodedDoc = "decoded:claim:" + id
				b.r.Documents[e.DecodedDoc] = prettyValue(view)
			}
			if prev := lastSeries[key]; prev != nil {
				pv, _ := b.view(prev)
				if diffs := diffValues(pv, view); len(diffs) > 0 {
					e.Changes = append(e.Changes, ChangeSet{What: "decoded", Label: title + " (vs previous claim in this series)",
						Against: prev.str("itemid"), AgainstDoc: "claim:" + prev.str("itemid"), AgainstT: ms(claimTime(prev)), Diffs: diffs})
				}
			}
		}
		b.finish(e.Changes, sumCtx{function: e.Function, pcrScheme: d.str("header.element.tpm2.pcrnaming"), names: b.c.pcrNames, decoded: view != nil})
		if bt := d.str("bodytype"); bt == "*ERROR" {
			e.Detail = join2("claim failed", e.Detail)
		}
		if !seenSeries[key] {
			seenSeries[key] = true
			b.r.Series = append(b.r.Series, Series{Key: key, Type: "claim", Intent: e.Intent, IntentID: pid, Endpoint: e.Endpoint})
		}
		b.add(e)
		lastElement, lastIntent[pid], lastSeries[key] = d, d, d
	}

	// the intents as they are now, against their last copy in a claim
	if !tr.includesNow(now) {
		return
	}
	for _, pid := range sortedKeys(c.intents) {
		cur := c.intents[pid]
		last := lastIntent[pid]
		if last == nil {
			continue
		}
		diffs := diffValues(last.get("header.intent"), cur.m)
		if len(diffs) == 0 {
			continue
		}
		b.add(Event{
			ID: "intent-current:" + pid, T: now.UnixMilli(), NS: strconv.FormatInt(now.UnixNano(), 10),
			Type: "intent", Kind: "current", ItemID: pid, Doc: b.addDoc("intent:"+pid, cur),
			Intent: b.intentName(pid), IntentID: pid,
			Title:  "Intent now: " + b.intentName(pid),
			Detail: "the intent in Jane now differs from its copy in the last claim",
			Changes: []ChangeSet{{What: "intent", Label: "Intent now (vs copy in last claim)",
				Against: last.str("itemid"), AgainstDoc: "claim:" + last.str("itemid"), AgainstT: ms(claimTime(last)), Diffs: diffs}},
		})
	}
}

// appendChange adds the differences of path between prev and cur, if any;
// with no prev and markFirst, a change set saying there is nothing to
// compare with.
func appendChange(cs []ChangeSet, what, label, typ string, prev, cur *doc, path string, markFirst bool) []ChangeSet {
	if prev == nil {
		if markFirst {
			cs = append(cs, ChangeSet{What: what, Label: label, First: true})
		}
		return cs
	}
	diffs := diffValues(prev.get(path), cur.get(path))
	if len(diffs) == 0 {
		return cs
	}
	return append(cs, ChangeSet{What: what, Label: label, Against: prev.str("itemid"),
		AgainstDoc: typ + ":" + prev.str("itemid"), AgainstT: ms(timeOf(prev)), Diffs: diffs})
}

// timeOf is a claim's or a result's time.
func timeOf(d *doc) int64 {
	if t := claimTime(d); t != 0 {
		return t
	}
	return resultTime(d)
}

// results: each result in the range, compared with the one before it in its
// series (the outcome) and with the one before it for the same expected value
// (the expected value copy).
func (b *builder) results(tr timeRange, now time.Time) {
	c := b.c
	lastSeries := map[string]*doc{}
	lastEV := map[string]*doc{}
	seenSeries := map[string]bool{}
	inRange := map[*doc]bool{}
	for _, d := range c.results {
		inRange[d] = true
	}
	all := byTime(append(append([]*doc{}, c.baseResults...), c.results...), resultTime)
	for _, d := range all {
		id := d.str("itemid")
		key := b.resultSeries(d)
		ev := d.str("expectedvalue.itemid")
		if !inRange[d] {
			b.addDoc("result:"+id, d)
			lastSeries[key] = d
			if ev != "" {
				lastEV[ev] = d
			}
			continue
		}
		code := d.num("result")
		pid, epn := splitSeries(key)
		e := Event{
			ID:         "result:" + id,
			T:          ms(resultTime(d)),
			NS:         strconv.FormatInt(resultTime(d), 10),
			Type:       "result",
			Kind:       "result",
			ItemID:     id,
			Doc:        b.addDoc("result:"+id, d),
			Intent:     b.intentName(pid),
			IntentID:   pid,
			Endpoint:   epn,
			Rule:       d.str("rulename"),
			EV:         ev,
			Series:     key,
			Result:     &code,
			ResultName: resultName(code),
			Session:    d.str("session.itemid"),
			Detail:     d.str("message"),
		}
		e.Title = fmt.Sprintf("Result: %s — %s", e.Rule, e.ResultName)
		e.RuleDesc = b.c.rules[e.Rule]
		if prev := lastSeries[key]; prev != nil {
			e.Prev = &Prev{ID: "result:" + prev.str("itemid"), T: ms(resultTime(prev)),
				Label: fmt.Sprintf("result of %s for %s on %s", e.Rule, e.Intent, epn)}
		}
		if prev := lastSeries[key]; prev != nil {
			if pc := prev.num("result"); pc != code {
				cs := ChangeSet{What: "outcome", Label: "Outcome (vs previous result of this rule, intent and endpoint)",
					Against: prev.str("itemid"), AgainstDoc: "result:" + prev.str("itemid"), AgainstT: ms(resultTime(prev)),
					Diffs: []Diff{{Path: "result", Op: "changed", Old: makeValue(resultName(pc)), New: makeValue(resultName(code))}}}
				if m := d.str("message"); m != "" {
					cs.Diffs = append(cs.Diffs, Diff{Path: "message", Op: "changed", Old: makeValue(prev.str("message")), New: makeValue(m)})
				}
				e.Changes = append(e.Changes, cs)
			}
		} else {
			e.Changes = append(e.Changes, ChangeSet{What: "outcome", Label: "Outcome (vs previous result of this rule, intent and endpoint)", First: true})
		}
		if ev != "" {
			e.Changes = appendChange(e.Changes, "expectedvalue", "Expected value copy (vs previous result with this expected value)", "result",
				lastEV[ev], d, "expectedvalue", true)
		}
		if !seenSeries[key] {
			seenSeries[key] = true
			b.r.Series = append(b.r.Series, Series{Key: key, Type: "result", Intent: e.Intent, IntentID: pid, Endpoint: epn, Rule: e.Rule})
		}
		b.add(e)
		lastSeries[key] = d
		if ev != "" {
			lastEV[ev] = d
		}
	}

	// the expected values as they are now, against their last copy in a result
	if !tr.includesNow(now) {
		return
	}
	for _, d := range c.evs {
		id := d.str("itemid")
		last := lastEV[id]
		if last == nil {
			continue
		}
		diffs := diffValues(last.get("expectedvalue"), d.m)
		if len(diffs) == 0 {
			continue
		}
		b.add(Event{
			ID: "ev-current:" + id, T: now.UnixMilli(), NS: strconv.FormatInt(now.UnixNano(), 10),
			Type: "ev", Kind: "current", ItemID: id, Doc: b.addDoc("ev:"+id, d),
			Intent: b.intentName(d.str("intentid")), IntentID: d.str("intentid"), Endpoint: d.str("endpointname"),
			Title:  "Expected value now: " + d.str("name"),
			Detail: "the expected value in Jane now differs from its copy in the last result",
			Changes: []ChangeSet{{What: "expectedvalue", Label: "Expected value now (vs copy in last result)",
				Against: last.str("itemid"), AgainstDoc: "result:" + last.str("itemid"), AgainstT: ms(resultTime(last)), Diffs: diffs}},
		})
	}
}

// resultSeries is collection.resultSeries without reading anything more.
func (b *builder) resultSeries(d *doc) string {
	pid, epn := d.str("expectedvalue.intentid"), d.str("expectedvalue.endpointname")
	if pid == "" || epn == "" {
		if claim := b.c.claimByID[d.str("claimid")]; claim != nil {
			pid, epn = claim.str("header.intent.itemid"), claim.str("header.endpointname")
		}
	}
	return pid + "|" + epn + "|" + d.str("rulename")
}

func splitSeries(key string) (pid, epn string) {
	parts := strings.SplitN(key, "|", 3)
	for len(parts) < 2 {
		parts = append(parts, "")
	}
	return parts[0], parts[1]
}

// element: when it was created and last updated, and, if the range reaches
// now, the element as it is now against its copy in the last claim.
func (b *builder) element(tr timeRange, now time.Time) {
	el := b.c.element
	id := el.str("itemid")
	key := b.addDoc("element:"+id, el)
	for _, k := range []struct{ field, kind, title string }{
		{"recordhistory.created", "created", "Element created"},
		{"recordhistory.lastupdated", "updated", "Element last updated"},
	} {
		ts := el.num(k.field)
		if ts == 0 || !tr.contains(ts) {
			continue
		}
		b.add(Event{ID: "element-" + k.kind + ":" + id, T: ms(ts), NS: strconv.FormatInt(ts, 10),
			Type: "element", Kind: k.kind, ItemID: id, Doc: key, Title: k.title + ": " + el.str("name"),
			Detail: el.str("recordhistory.reason")})
	}
	if !tr.includesNow(now) {
		return
	}
	e := Event{ID: "element-current:" + id, T: now.UnixMilli(), NS: strconv.FormatInt(now.UnixNano(), 10),
		Type: "element", Kind: "current", ItemID: id, Doc: key, Title: "Element now: " + el.str("name")}
	var last *doc
	all := byTime(append(append([]*doc{}, b.c.baseClaims...), b.c.claims...), claimTime)
	if len(all) > 0 {
		last = all[len(all)-1]
	}
	if last == nil {
		e.Detail = "no claim to compare the element with"
	} else {
		diffs := diffValues(last.get("header.element"), el.m)
		if len(diffs) == 0 {
			e.Detail = "unchanged since its copy in the last claim"
		} else {
			e.Detail = "the element in Jane now differs from its copy in the last claim"
			e.Changes = []ChangeSet{{What: "element", Label: "Element now (vs copy in last claim)",
				Against: last.str("itemid"), AgainstDoc: "claim:" + last.str("itemid"), AgainstT: ms(claimTime(last)), Diffs: diffs}}
		}
	}
	b.add(e)
}

// expectedValues: when each was created and last updated.
func (b *builder) expectedValues(tr timeRange) {
	for _, d := range b.c.evs {
		id := d.str("itemid")
		key := b.addDoc("ev:"+id, d)
		pid := d.str("intentid")
		for _, k := range []struct{ field, kind, title string }{
			{"recordhistory.created", "created", "Expected value created"},
			{"recordhistory.lastupdated", "updated", "Expected value last updated"},
		} {
			ts := d.num(k.field)
			if ts == 0 || !tr.contains(ts) {
				continue
			}
			b.add(Event{ID: "ev-" + k.kind + ":" + id, T: ms(ts), NS: strconv.FormatInt(ts, 10),
				Type: "ev", Kind: k.kind, ItemID: id, Doc: key,
				Intent: b.intentName(pid), IntentID: pid, Endpoint: d.str("endpointname"),
				Title:  fmt.Sprintf("%s: %s (%s on %s)", k.title, d.str("name"), b.intentName(pid), d.str("endpointname")),
				Detail: d.str("recordhistory.reason")})
		}
	}
	for id, d := range b.c.intents {
		b.addDoc("intent:"+id, d)
	}
}

// sessions: opened and closed, if in the range.
func (b *builder) sessions(tr timeRange) {
	for _, id := range sortedKeys(b.c.sessions) {
		d := b.c.sessions[id]
		key := b.addDoc("session:"+id, d)
		claims, _ := d.get("claimlist").([]any)
		results, _ := d.get("resultlist").([]any)
		detail := fmt.Sprintf("%d claims, %d results", len(claims), len(results))
		if p := d.str("parent"); p != "" {
			detail += ", parent session " + p
		}
		for _, k := range []struct{ field, kind, title string }{
			{"timing.opened", "opened", "Session opened"},
			{"timing.closed", "closed", "Session closed"},
		} {
			ts := d.num(k.field)
			if ts == 0 || !tr.contains(ts) {
				continue
			}
			title := k.title
			if m := d.str("message"); m != "" {
				title += ": " + m
			}
			b.add(Event{ID: "session-" + k.kind + ":" + id, T: ms(ts), NS: strconv.FormatInt(ts, 10),
				Type: "session", Kind: k.kind, ItemID: id, Doc: key, Session: id, Title: title, Detail: detail})
		}
	}
}

func (b *builder) logEntries() {
	for _, d := range b.c.logs {
		id := d.str("itemid")
		ts := d.num("timestamp")
		e := Event{ID: "log:" + id, T: ms(ts), NS: strconv.FormatInt(ts, 10),
			Type: "log", Kind: "log", ItemID: id, Doc: b.addDoc("log:"+id, d),
			Title:  fmt.Sprintf("Log: %s %s %s", d.str("channel"), d.str("operation"), d.str("reftype")),
			Detail: d.str("message")}
		if ref := d.str("refid"); ref != "" {
			e.Refs = append(e.Refs, Ref{Label: d.str("reftype") + " " + ref, ID: ref})
		}
		b.add(e)
	}
}

// links fills in each event's references: claims to their results and
// session, results to their claim and session, sessions to their claims and
// results, log entries to what they are about. A reference is to an event
// where there is one, else to a document.
func (b *builder) links() {
	resultsOf := map[string][]string{}
	for _, d := range b.c.results {
		resultsOf[d.str("claimid")] = append(resultsOf[d.str("claimid")], d.str("itemid"))
	}
	target := func(typ, id string) string {
		for _, k := range []string{typ + ":" + id, typ + "-opened:" + id, typ + "-current:" + id, typ + "-created:" + id} {
			if b.events[k] {
				return k
			}
		}
		if _, ok := b.r.Documents[typ+":"+id]; ok {
			return typ + ":" + id
		}
		return ""
	}
	for i := range b.r.Events {
		e := &b.r.Events[i]
		switch e.Type {
		case "claim":
			for _, rid := range resultsOf[e.ItemID] {
				e.Refs = append(e.Refs, Ref{Label: "result " + rid, ID: target("result", rid)})
			}
			if e.Session != "" {
				e.Refs = append(e.Refs, Ref{Label: "session " + e.Session, ID: target("session", e.Session)})
			}
		case "result":
			d := findDoc(b.c.results, e.ItemID)
			cid := d.str("claimid")
			if cid != "" {
				e.Refs = append(e.Refs, Ref{Label: "claim " + cid, ID: target("claim", cid)})
			}
			if ev := d.str("expectedvalue.itemid"); ev != "" {
				e.Refs = append(e.Refs, Ref{Label: "expected value " + ev, ID: target("ev", ev)})
			}
			// claims named in the rule's parameters (pcrscid, uefilogcid, quotecid, previouscid, ...)
			if ps, ok := d.get("parameters").(map[string]any); ok {
				for _, k := range sortedKeys(ps) {
					v, _ := ps[k].(string)
					if v == "" || v == cid {
						continue
					}
					if t := target("claim", v); t != "" {
						e.Refs = append(e.Refs, Ref{Label: k + " → claim " + v, ID: t})
					} else if _, known := b.c.claimByID[v]; known {
						e.Refs = append(e.Refs, Ref{Label: k + " → claim " + v})
					}
				}
			}
			if e.Session != "" {
				e.Refs = append(e.Refs, Ref{Label: "session " + e.Session, ID: target("session", e.Session)})
			}
		case "session":
			d := b.c.sessions[e.ItemID]
			for _, f := range []struct{ list, typ string }{{"claimlist", "claim"}, {"resultlist", "result"}} {
				items, _ := d.get(f.list).([]any)
				for _, x := range items {
					if s, ok := x.(string); ok {
						if t := target(f.typ, s); t != "" {
							e.Refs = append(e.Refs, Ref{Label: f.typ + " " + s, ID: t})
						}
					}
				}
			}
		case "log":
			for j := range e.Refs {
				ref := e.Refs[j].ID
				e.Refs[j].ID = ""
				for _, typ := range []string{"claim", "result", "session", "ev", "element"} {
					if t := target(typ, ref); t != "" {
						e.Refs[j].ID = t
						break
					}
				}
			}
		}
	}
}

func findDoc(ds []*doc, id string) *doc {
	for _, d := range ds {
		if d.str("itemid") == id {
			return d
		}
	}
	return nil
}

func join2(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// view is a claim's decoded view and its title, decoded once per claim.
func (b *builder) view(d *doc) (map[string]any, string) {
	if v, ok := b.views[d]; ok {
		return v.view, v.title
	}
	body, _ := d.get("body").(map[string]any)
	v, title := decodedView(d.str("header.intent.function"), body)
	b.views[d] = cachedView{v, title}
	return v, title
}

type cachedView struct {
	view  map[string]any
	title string
}

// finish marks the volatile differences of claim bodies and writes each
// change set's summary.
func (b *builder) finish(cs []ChangeSet, ctx sumCtx) {
	for i := range cs {
		if cs[i].What == "body" {
			for j := range cs[i].Diffs {
				cs[i].Diffs[j].V = isVolatile(ctx.function, cs[i].Diffs[j].Path)
			}
		}
		cs[i].Summary = summarize(cs[i].What, cs[i].Diffs, ctx)
	}
}

// prettyValue writes a decoded view as indented JSON.
func prettyValue(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// hasChange reports whether an event has a difference that is not volatile.
func hasChange(e Event) bool {
	for _, cs := range e.Changes {
		for _, d := range cs.Diffs {
			if !d.V {
				return true
			}
		}
	}
	return false
}
