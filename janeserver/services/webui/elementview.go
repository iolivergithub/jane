package webui

import (
	"sort"
	"time"

	"a10/structures"
)

// View model for the single element page: the element's claims and results
// grouped into attestation sessions, plus the data behind the timeline charts
// and the rule/claim history matrix.

// How much history the element page fetches and how many sessions it plots.
const (
	elementClaimsFetched    = 300
	elementResultsFetched   = 600
	elementTimelineSessions = 30
)

// Outcome classes shared by the charts, the matrix and the lists.
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

// Outcome is the overall verdict for the session: fail if any rule failed,
// other if anything did not verify cleanly or a claim could not be collected,
// pass if everything passed.
func (s elementSession) Outcome() string {
	switch {
	case s.Fail > 0:
		return outcomeFail
	case s.Other > 0 || s.ClaimErrors > 0:
		return outcomeOther
	case s.Pass > 0:
		return outcomePass
	default:
		return ""
	}
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

// timelineChart is serialised into the page for Chart.js (html/template
// JSON-encodes it in the script context).
type timelineChart struct {
	Labels     []string `json:"labels"`
	SessionIDs []string `json:"sessionIds"`
	Pass       []int    `json:"pass"`
	Fail       []int    `json:"fail"`
	Other      []int    `json:"other"`
	Claims     []int    `json:"claims"`
	ClaimErr   []int    `json:"claimErr"`
}

// elementPage is what element.html renders. E, CS and RS keep their old names.
type elementPage struct {
	E  structures.Element
	CS []structures.Claim  // newest first
	RS []structures.Result // newest first

	Sessions []elementSession // newest first
	Timeline []elementSession // oldest first, at most elementTimelineSessions
	Latest   *elementSession

	RuleRows  []matrixRow
	ClaimRows []matrixRow
	Chart     timelineChart

	NPass, NFail, NOther, NClaimErrors int
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

	// timeline: the most recent sessions, oldest first (time runs left to right)
	n := len(p.Sessions)
	if n > elementTimelineSessions {
		n = elementTimelineSessions
	}
	for i := n - 1; i >= 0; i-- {
		p.Timeline = append(p.Timeline, p.Sessions[i])
	}

	for _, s := range p.Timeline {
		p.Chart.Labels = append(p.Chart.Labels, s.Label)
		p.Chart.SessionIDs = append(p.Chart.SessionIDs, s.ID)
		p.Chart.Pass = append(p.Chart.Pass, s.Pass)
		p.Chart.Fail = append(p.Chart.Fail, s.Fail)
		p.Chart.Other = append(p.Chart.Other, s.Other)
		p.Chart.Claims = append(p.Chart.Claims, len(s.Claims)-s.ClaimErrors)
		p.Chart.ClaimErr = append(p.Chart.ClaimErr, s.ClaimErrors)
	}

	p.RuleRows = buildMatrix(p.Timeline, func(s elementSession, add func(label, outcome, link, title string)) {
		for _, r := range s.Results {
			title := r.RuleName + " · " + s.Label + " · " + resultOutcome(r.Result)
			if r.Message != "" {
				title += " · " + r.Message
			}
			add(r.RuleName, resultOutcome(r.Result), "/result/"+r.ItemID, title)
		}
	})
	p.ClaimRows = buildMatrix(p.Timeline, func(s elementSession, add func(label, outcome, link, title string)) {
		for _, c := range s.Claims {
			status := "collected"
			if claimOutcome(c) == outcomeClaimErr {
				status = "error"
			}
			add(claimLabel(c), claimOutcome(c), "/claim/"+c.ItemID, claimLabel(c)+" · "+s.Label+" · "+status)
		}
	})

	return p
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
