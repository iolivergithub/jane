package webui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"a10/operations"
	"a10/structures"
)

// View model for the single element page: the element's claims and results
// grouped into attestation sessions, and the rule/claim history matrix.

// How much history the element page fetches and how many sessions the history matrix shows.
const (
	elementClaimsFetched    = 300
	elementResultsFetched   = 600
	elementTimelineSessions = 30
	elementListLimit        = 100 // rows per list at the bottom of the page
)

// Outcome classes shared by the matrix and the lists.
const (
	outcomePass     = "pass"
	outcomeFail     = "fail"
	outcomeOther    = "other"
	outcomeClaim    = "claim"
	outcomeClaimErr = "claimerr"
)

// resultOutcome maps a result value to pass / fail / other.
func resultOutcome(v structures.ResultValue) string {
	switch v {
	case structures.Success:
		return outcomePass
	case structures.Fail:
		return outcomeFail
	default:
		return outcomeOther
	}
}

// claimOutcome maps a claim to claim / claimerr.
func claimOutcome(c structures.Claim) string {
	if c.BodyType == structures.CLAIMERROR {
		return outcomeClaimErr
	}
	return outcomeClaim
}

// severity orders outcomes so a matrix cell holding several items shows the worst.
var severity = map[string]int{outcomeClaim: 0, outcomePass: 0, outcomeOther: 1, outcomeClaimErr: 2, outcomeFail: 2}

// elementSession is one attestation session as seen from one element.
type elementSession struct {
	ID       string
	Opened   structures.Timestamp // from the session record, else earliest claim/result
	Closed   structures.Timestamp
	Found    bool   // the session record itself was found
	Duration string // empty while the session is open or unknown
	Message  string
	Label    string // short "MM-DD HH:MM" UTC label for axes

	Claims  []structures.Claim
	Results []structures.Result

	Pass, Fail, Other          int
	ClaimErrors                int
	PassPct, FailPct, OtherPct float64 // for the inline stacked bar in the list
}

// matrixCell is one rule (or claim intent) in one session of the timeline.
type matrixCell struct {
	Outcome string // empty when the rule/intent was not evaluated in that session
	Link    string
	Title   string
}

type matrixRow struct {
	Label string
	Cells []matrixCell
}

// elementPage is what element.html renders. E, CS and RS keep their old names.
type elementPage struct {
	E  structures.Element
	CS []structures.Claim  // newest first
	RS []structures.Result // newest first

	Sessions []elementSession // newest first
	Timeline []elementSession // history matrix columns: oldest first, at most elementTimelineSessions
	Latest   *elementSession

	RuleRows   []matrixRow
	RuleGroups []ruleGroup // RuleRows grouped by rule type, for the collapsible history matrix
	ClaimRows  []matrixRow

	NPass, NFail, NOther, NClaimErrors int

	// The lists at the bottom of the page, each capped at elementListLimit.
	ListSessions []elementSession
	ListResults  []structures.Result
	ListClaims   []structures.Claim

	// Relationships (#118): this element's, with the targets' names, and
	// those other elements have to this one. Display only.
	Related     []elementRelation
	RelatedFrom []operations.RelatedFrom
}

// elementRelation is one of the element's relationships, with the name of
// the target element; Missing if that element no longer exists.
type elementRelation struct {
	RelationshipType string
	ItemID           string
	Name             string
	Missing          bool
}

func shortLabel(t structures.Timestamp) string {
	return time.Unix(0, int64(t)).UTC().Format("01-02 15:04")
}

func formatDuration(opened, closed structures.Timestamp) string {
	if opened == 0 || closed == 0 || closed < opened {
		return ""
	}
	d := time.Duration(closed - opened)
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(10 * time.Millisecond).String()
	default:
		return d.Round(time.Second).String()
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}

func claimLabel(c structures.Claim) string {
	if c.Header.Intent.Name != "" {
		return c.Header.Intent.Name
	}
	return c.BodyType
}

// buildElementPage groups claims and results (both newest first) into
// sessions. lookup returns the stored session record; it is called once per
// distinct session and may be nil (then timings come from claims/results).
func buildElementPage(e structures.Element, cs []structures.Claim, rs []structures.Result,
	lookup func(id string) (structures.Session, bool)) elementPage {

	p := elementPage{E: e, CS: cs, RS: rs}

	byID := map[string]*elementSession{}
	latestActivity := map[string]structures.Timestamp{}
	var order []string

	get := func(id string, t structures.Timestamp) *elementSession {
		s, ok := byID[id]
		if !ok {
			s = &elementSession{ID: id}
			byID[id] = s
			order = append(order, id)
		}
		if t > latestActivity[id] {
			latestActivity[id] = t
		}
		if t != 0 && (s.Opened == 0 || t < s.Opened) {
			s.Opened = t // provisional, replaced by the session record below
		}
		return s
	}

	for _, c := range cs {
		s := get(c.Header.Session.ItemID, c.Header.Timing.Requested)
		s.Claims = append(s.Claims, c)
		if claimOutcome(c) == outcomeClaimErr {
			s.ClaimErrors++
			p.NClaimErrors++
		}
	}
	for _, r := range rs {
		s := get(r.Session.ItemID, r.VerifiedAt)
		s.Results = append(s.Results, r)
		switch resultOutcome(r.Result) {
		case outcomePass:
			s.Pass++
			p.NPass++
		case outcomeFail:
			s.Fail++
			p.NFail++
		default:
			s.Other++
			p.NOther++
		}
	}

	for _, id := range order {
		s := byID[id]
		if lookup != nil && id != "" {
			if rec, ok := lookup(id); ok {
				s.Found = true
				if rec.Timing.Opened != 0 {
					s.Opened = rec.Timing.Opened
				}
				s.Closed = rec.Timing.Closed
				s.Message = rec.Message
			}
		}
		s.Duration = formatDuration(s.Opened, s.Closed)
		s.Label = shortLabel(s.Opened)
		total := s.Pass + s.Fail + s.Other
		s.PassPct, s.FailPct, s.OtherPct = pct(s.Pass, total), pct(s.Fail, total), pct(s.Other, total)
		sort.SliceStable(s.Results, func(a, b int) bool { return s.Results[a].RuleName < s.Results[b].RuleName })
		p.Sessions = append(p.Sessions, *s)
	}

	// newest first by opening time, falling back to latest activity
	sort.SliceStable(p.Sessions, func(a, b int) bool {
		ta, tb := p.Sessions[a].Opened, p.Sessions[b].Opened
		if ta == tb {
			return latestActivity[p.Sessions[a].ID] > latestActivity[p.Sessions[b].ID]
		}
		return ta > tb
	})
	if len(p.Sessions) > 0 {
		p.Latest = &p.Sessions[0]
	}

	// history matrix columns: the most recent sessions, oldest first (time runs left to right)
	n := len(p.Sessions)
	if n > elementTimelineSessions {
		n = elementTimelineSessions
	}
	for i := n - 1; i >= 0; i-- {
		p.Timeline = append(p.Timeline, p.Sessions[i])
	}

	// One row per rule and intent: the same rule applied to claims of different
	// intents gets a row each instead of being merged into one (#53).
	p.RuleRows = buildMatrix(p.Timeline, func(s elementSession, add func(label, outcome, link, title string)) {
		intentOf := map[string]string{}
		for _, c := range s.Claims {
			intentOf[c.ItemID] = claimLabel(c)
		}
		for _, r := range s.Results {
			label := ruleRowLabel(r.RuleName, intentOf[r.ClaimID])
			title := r.RuleName
			if in := intentOf[r.ClaimID]; in != "" {
				title += " · intent " + in
			}
			title += " · " + s.Label + " · " + resultOutcome(r.Result)
			if r.Message != "" {
				title += " · " + r.Message
			}
			add(label, resultOutcome(r.Result), "/result/"+r.ItemID, title)
		}
	})
	p.RuleGroups = groupRuleRows(p.RuleRows, p.Timeline)
	p.ClaimRows = buildMatrix(p.Timeline, func(s elementSession, add func(label, outcome, link, title string)) {
		for _, c := range s.Claims {
			status := "collected"
			if claimOutcome(c) == outcomeClaimErr {
				status = "error"
			}
			add(claimLabel(c), claimOutcome(c), "/claim/"+c.ItemID, claimLabel(c)+" · "+s.Label+" · "+status)
		}
	})

	p.ListSessions = firstN(p.Sessions, elementListLimit)
	p.ListResults = firstN(p.RS, elementListLimit)
	p.ListClaims = firstN(p.CS, elementListLimit)

	return p
}

// ruleRowLabel names a history matrix row: the rule, and the intent of the
// claim it was applied to when that is known.
func ruleRowLabel(rule, intent string) string {
	if intent == "" {
		return rule
	}
	return rule + " · " + intent
}

// firstN returns at most the first n items of xs.
func firstN[T any](xs []T, n int) []T {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// buildMatrix makes one row per label (sorted) and one cell per timeline
// session; when a label occurs more than once in a session the worst outcome wins.
func buildMatrix(timeline []elementSession,
	each func(s elementSession, add func(label, outcome, link, title string))) []matrixRow {

	rows := map[string][]matrixCell{}
	for col, s := range timeline {
		each(s, func(label, outcome, link, title string) {
			cells, ok := rows[label]
			if !ok {
				cells = make([]matrixCell, len(timeline))
				rows[label] = cells
			}
			cur := cells[col]
			if cur.Outcome == "" || severity[outcome] > severity[cur.Outcome] {
				cells[col] = matrixCell{Outcome: outcome, Link: link, Title: title}
			}
		})
	}

	labels := make([]string, 0, len(rows))
	for l := range rows {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	out := make([]matrixRow, 0, len(labels))
	for _, l := range labels {
		out = append(out, matrixRow{Label: l, Cells: rows[l]})
	}
	return out
}

// ruleGroup is one type of rule in the history matrix (#59): the rows whose
// rule name shares a prefix (tpm2_, sys_, UEFI_, ...), shown collapsed as a
// single summary row holding the worst outcome per session.
type ruleGroup struct {
	Name    string // the prefix as written in the first rule name seen, e.g. "tpm2", "UEFI"
	ID      string // HTML id fragment, unique within the page
	Summary []matrixCell
	Rows    []matrixRow
}

// ruleType returns the prefix of a rule name: the part before the first "_",
// or "other" when there is none. A row label may carry " · intent" after the
// rule name; that does not affect the prefix.
func ruleType(label string) string {
	if i := strings.Index(label, "_"); i > 0 {
		return label[:i]
	}
	return "other"
}

// groupRuleRows groups rows by rule type, case-insensitively, with groups and
// the rows within them in case-insensitive alphabetical order (as on the
// attest page, so UEFI_db comes before UEFI_PK). Each summary cell is the
// worst outcome of the group in that session (fail, then other, then pass) and
// links to the session.
func groupRuleRows(rows []matrixRow, timeline []elementSession) []ruleGroup {
	byKey := map[string]*ruleGroup{}
	var keys []string
	for _, r := range rows {
		name := ruleType(r.Label)
		key := strings.ToLower(name)
		g, ok := byKey[key]
		if !ok {
			g = &ruleGroup{Name: name}
			byKey[key] = g
			keys = append(keys, key)
		}
		g.Rows = append(g.Rows, r)
	}
	sort.Strings(keys)

	out := make([]ruleGroup, 0, len(keys))
	for i, key := range keys {
		g := byKey[key]
		g.ID = fmt.Sprintf("rg%d", i)
		sort.SliceStable(g.Rows, func(a, b int) bool {
			return strings.ToLower(g.Rows[a].Label) < strings.ToLower(g.Rows[b].Label)
		})
		g.Summary = make([]matrixCell, len(timeline))
		for col, s := range timeline {
			counts := map[string]int{}
			worst := ""
			for _, r := range g.Rows {
				o := r.Cells[col].Outcome
				if o == "" {
					continue
				}
				counts[o]++
				if worst == "" || severity[o] > severity[worst] {
					worst = o
				}
			}
			if worst == "" {
				continue
			}
			g.Summary[col] = matrixCell{
				Outcome: worst,
				Link:    "/session/" + s.ID,
				Title: fmt.Sprintf("%s · %s · %d pass, %d fail, %d other",
					g.Name, s.Label, counts[outcomePass], counts[outcomeFail], counts[outcomeOther]),
			}
		}
		out = append(out, *g)
	}
	return out
}
