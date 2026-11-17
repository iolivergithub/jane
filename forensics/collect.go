package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// collection is everything read from Jane for one element.
type collection struct {
	element *doc
	evs     []*doc          // the element's expected values, as they are now
	intents map[string]*doc // the intents used, as they are now, by itemid

	claims      []*doc // in the range, oldest first
	baseClaims  []*doc // just before the range, for the first comparisons; oldest first
	results     []*doc // in the range, oldest first
	baseResults []*doc
	claimByID   map[string]*doc // every claim read, including baselines
	sessions    map[string]*doc // sessions of the claims and results in the range
	logs        []*doc          // log entries in the range about any of the above, oldest first

	listed   map[string]int // how many itemids Jane listed: claims, results, log
	warnings []string

	pcrNames *pcrNames         // Jane's register naming schemes, for naming PCRs
	rules    map[string]string // Jane's rules: name -> description
}

func (c *collection) warn(format string, a ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, a...))
}

func claimTime(d *doc) int64  { return d.num("header.timing.requested") }
func resultTime(d *doc) int64 { return d.num("verifiedat") }

// claimSeries identifies a claim's series: the same intent on the same
// endpoint.
func claimSeries(d *doc) string {
	return d.str("header.intent.itemid") + "|" + d.str("header.endpointname")
}

// collect reads the element and everything belonging to it from Jane.
// Progress goes to w.
func collect(cl *client, eid string, tr timeRange, max int64, now time.Time, w io.Writer) (*collection, error) {
	c := &collection{
		intents:   map[string]*doc{},
		claimByID: map[string]*doc{},
		sessions:  map[string]*doc{},
		listed:    map[string]int{},
	}

	fmt.Fprintf(w, "jane_forensics: element %s\n", eid)
	el, err := cl.getDoc("/element/" + esc(eid))
	var se *statusError
	if errors.As(err, &se) {
		return nil, fmt.Errorf("element %s not found (Jane answered %d: %s)", eid, se.status, se.body)
	}
	if err != nil {
		return nil, fmt.Errorf("reading element %s: %w", eid, err)
	}
	if el.str("itemid") != eid {
		return nil, fmt.Errorf("element %s not found (Jane answered with itemid %q)", eid, el.str("itemid"))
	}
	c.element = el
	c.readNames(cl)

	// expected values, as they are now
	evIDs, err := cl.getList("/expectedValues/element/"+esc(eid), "expectedValues")
	if err != nil {
		return nil, fmt.Errorf("listing expected values: %w", err)
	}
	for _, id := range evIDs {
		d, err := cl.getDoc("/expectedValue/" + esc(id))
		if err != nil {
			c.warn("expected value %s: %v", id, err)
			continue
		}
		c.evs = append(c.evs, d)
	}
	fmt.Fprintf(w, "jane_forensics: %d expected values\n", len(c.evs))

	if err := c.collectClaims(cl, eid, tr, max, w); err != nil {
		return nil, err
	}
	if err := c.collectResults(cl, eid, tr, max, w); err != nil {
		return nil, err
	}

	// intents named by the claims and expected values, as they are now
	intentIDs := map[string]bool{}
	for _, d := range append(append([]*doc{}, c.claims...), c.baseClaims...) {
		if id := d.str("header.intent.itemid"); id != "" {
			intentIDs[id] = true
		}
	}
	for _, d := range c.evs {
		if id := d.str("intentid"); id != "" {
			intentIDs[id] = true
		}
	}
	for _, id := range sortedKeys(intentIDs) {
		d, err := cl.getDoc("/intent/" + esc(id))
		if err != nil {
			c.warn("intent %s could not be read (removed?): %v", id, err)
			continue
		}
		c.intents[id] = d
	}
	fmt.Fprintf(w, "jane_forensics: %d intents\n", len(c.intents))

	// sessions of the claims and results in the range
	sessionIDs := map[string]bool{}
	for _, d := range c.claims {
		if id := d.str("header.session.itemid"); id != "" {
			sessionIDs[id] = true
		}
	}
	for _, d := range c.results {
		if id := d.str("session.itemid"); id != "" {
			sessionIDs[id] = true
		}
	}
	for _, id := range sortedKeys(sessionIDs) {
		d, err := cl.getDoc("/session/" + esc(id))
		if err != nil {
			c.warn("session %s: %v", id, err)
			continue
		}
		c.sessions[id] = d
	}
	fmt.Fprintf(w, "jane_forensics: %d sessions\n", len(c.sessions))

	if err := c.collectLog(cl, eid, tr, max, now, w); err != nil {
		return nil, err
	}
	return c, nil
}

// collectClaims reads the element's claims, newest first as Jane lists them,
// keeping those in the range and, before it, one baseline claim for the
// element copy, for each intent and for each series that appears in the
// range. It stops at the first claim before the range that is not needed.
func (c *collection) collectClaims(cl *client, eid string, tr timeRange, max int64, w io.Writer) error {
	ids, err := cl.getList(fmt.Sprintf("/claims/element/%s?max=%d", esc(eid), max), "claims")
	if err != nil {
		return fmt.Errorf("listing claims: %w", err)
	}
	c.listed["claims"] = len(ids)
	if int64(len(ids)) >= max {
		c.warn("Jane listed %d claims, the maximum asked for (--max): older claims may be missing", len(ids))
	}

	need := map[string]bool{} // "element", "intent:<id>", "series:<key>"
	have := map[string]bool{}
	outstanding := func() bool {
		for k := range need {
			if !have[k] {
				return true
			}
		}
		return false
	}
	var inRange, base []*doc
	for i, id := range ids {
		if i > 0 && i%200 == 0 {
			fmt.Fprintf(w, "jane_forensics: claims: read %d of %d\n", i, len(ids))
		}
		d, err := cl.getDoc("/claim/" + esc(id))
		if err != nil {
			c.warn("claim %s: %v", id, err)
			continue
		}
		ts := claimTime(d)
		switch {
		case tr.after(ts):
			continue
		case !tr.before(ts):
			inRange = append(inRange, d)
			c.claimByID[id] = d
			need["element"] = true
			need["intent:"+d.str("header.intent.itemid")] = true
			need["series:"+claimSeries(d)] = true
			continue
		}
		// before the range: a baseline, if it is still needed
		keys := []string{"element", "intent:" + d.str("header.intent.itemid"), "series:" + claimSeries(d)}
		useful := false
		for _, k := range keys {
			if need[k] && !have[k] {
				useful = true
				have[k] = true
			}
		}
		if useful {
			base = append(base, d)
			c.claimByID[id] = d
		}
		if !outstanding() {
			break
		}
	}
	c.claims = byTime(inRange, claimTime)
	c.baseClaims = byTime(base, claimTime)
	fmt.Fprintf(w, "jane_forensics: %d claims in the range, %d baseline claims before it (of %d listed)\n",
		len(c.claims), len(c.baseClaims), len(ids))
	return nil
}

// resultSeries identifies a result's series: the intent and endpoint of its
// claim and the rule. They are taken from the result's expected value copy,
// else from its claim.
func (c *collection) resultSeries(cl *client, d *doc) string {
	pid, epn := d.str("expectedvalue.intentid"), d.str("expectedvalue.endpointname")
	if pid == "" || epn == "" {
		if claim := c.claimOf(cl, d); claim != nil {
			pid, epn = claim.str("header.intent.itemid"), claim.str("header.endpointname")
		}
	}
	return pid + "|" + epn + "|" + d.str("rulename")
}

// claimOf returns the claim a result is about, reading it if needed.
func (c *collection) claimOf(cl *client, d *doc) *doc {
	id := d.str("claimid")
	if id == "" {
		return nil
	}
	if claim, ok := c.claimByID[id]; ok {
		return claim
	}
	claim, err := cl.getDoc("/claim/" + esc(id))
	if err != nil {
		c.claimByID[id] = nil
		return nil
	}
	c.claimByID[id] = claim
	return claim
}

// collectResults reads the element's results as collectClaims reads claims,
// with baselines for each result series and each expected value.
func (c *collection) collectResults(cl *client, eid string, tr timeRange, max int64, w io.Writer) error {
	ids, err := cl.getList(fmt.Sprintf("/results/element/%s?max=%d", esc(eid), max), "results")
	if err != nil {
		return fmt.Errorf("listing results: %w", err)
	}
	c.listed["results"] = len(ids)
	if int64(len(ids)) >= max {
		c.warn("Jane listed %d results, the maximum asked for (--max): older results may be missing", len(ids))
	}

	need := map[string]bool{}
	have := map[string]bool{}
	outstanding := func() bool {
		for k := range need {
			if !have[k] {
				return true
			}
		}
		return false
	}
	var inRange, base []*doc
	for i, id := range ids {
		if i > 0 && i%200 == 0 {
			fmt.Fprintf(w, "jane_forensics: results: read %d of %d\n", i, len(ids))
		}
		d, err := cl.getDoc("/result/" + esc(id))
		if err != nil {
			c.warn("result %s: %v", id, err)
			continue
		}
		ts := resultTime(d)
		if tr.after(ts) {
			continue
		}
		keys := []string{"series:" + c.resultSeries(cl, d)}
		if ev := d.str("expectedvalue.itemid"); ev != "" {
			keys = append(keys, "ev:"+ev)
		}
		if !tr.before(ts) {
			inRange = append(inRange, d)
			for _, k := range keys {
				need[k] = true
			}
			continue
		}
		useful := false
		for _, k := range keys {
			if need[k] && !have[k] {
				useful = true
				have[k] = true
			}
		}
		if useful {
			base = append(base, d)
		}
		if !outstanding() {
			break
		}
	}
	c.results = byTime(inRange, resultTime)
	c.baseResults = byTime(base, resultTime)
	fmt.Fprintf(w, "jane_forensics: %d results in the range, %d baseline results before it (of %d listed)\n",
		len(c.results), len(c.baseResults), len(ids))
	return nil
}

// collectLog reads the log entries in the range and keeps those about the
// element, its expected values, or the claims, results and sessions in the
// range. Entries about intents are left out: intents are shared with other
// elements.
func (c *collection) collectLog(cl *client, eid string, tr timeRange, max int64, now time.Time, w io.Writer) error {
	var path string
	if tr.All {
		path = fmt.Sprintf("/log?max=%d", max)
	} else {
		// everything since the start of the range, with a minute to spare
		since := now.Sub(tr.From) + time.Minute
		path = fmt.Sprintf("/log/since?duration=%ds", int64(since.Seconds())+1)
	}
	entries, err := cl.getLogEntries(path)
	if err != nil {
		return fmt.Errorf("reading the log: %w", err)
	}
	c.listed["log"] = len(entries)
	if tr.All && int64(len(entries)) >= max {
		c.warn("Jane returned %d log entries, the maximum asked for (--max): older entries may be missing", len(entries))
	}

	related := map[string]bool{eid: true}
	for _, d := range c.evs {
		related[d.str("itemid")] = true
	}
	for _, d := range c.claims {
		related[d.str("itemid")] = true
	}
	for _, d := range c.results {
		related[d.str("itemid")] = true
	}
	for id := range c.sessions {
		related[id] = true
	}
	var kept []*doc
	for _, d := range entries {
		if related[d.str("refid")] && tr.contains(d.num("timestamp")) {
			kept = append(kept, d)
		}
	}
	c.logs = byTime(kept, func(d *doc) int64 { return d.num("timestamp") })
	fmt.Fprintf(w, "jane_forensics: %d log entries about the element (of %d read)\n", len(c.logs), len(entries))
	return nil
}

// byTime sorts documents oldest first, by itemid where the times are equal.
func byTime(ds []*doc, t func(*doc) int64) []*doc {
	sort.SliceStable(ds, func(i, j int) bool {
		ti, tj := t(ds[i]), t(ds[j])
		if ti != tj {
			return ti < tj
		}
		return strings.Compare(ds[i].str("itemid"), ds[j].str("itemid")) < 0
	})
	return ds
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// readNames reads Jane's register naming schemes and rule descriptions, to
// name PCRs and rules in the report. Without them the report has numbers
// and rule names only.
func (c *collection) readNames(cl *client) {
	var pn struct {
		Default string `json:"default"`
		Schemes []struct {
			Key       string `json:"key"`
			Registers []struct {
				Register string `json:"register"`
				Short    string `json:"short"`
			} `json:"registers"`
		} `json:"schemes"`
	}
	if b, err := cl.get("/pcrnames"); err != nil {
		c.warn("register names could not be read (PCRs are shown by number only): %v", err)
	} else if err := json.Unmarshal(b, &pn); err != nil {
		c.warn("register names: %v", err)
	} else {
		c.pcrNames = &pcrNames{def: pn.Default, schemes: map[string]map[string]string{}}
		for _, s := range pn.Schemes {
			m := map[string]string{}
			for _, r := range s.Registers {
				m[r.Register] = r.Short
			}
			c.pcrNames.schemes[s.Key] = m
		}
	}

	var rs struct {
		Rules []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"rules"`
	}
	c.rules = map[string]string{}
	if b, err := cl.get("/rules"); err != nil {
		c.warn("rules could not be read (no rule descriptions): %v", err)
	} else if err := json.Unmarshal(b, &rs); err == nil {
		for _, r := range rs.Rules {
			c.rules[r.Name] = r.Description
		}
	}
}
