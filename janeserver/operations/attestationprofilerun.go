package operations

// Running an attestation profile (#129, #133). A request runs one profile on
// one element and, through the profile's follow entries, the profiles of
// related elements:
//
//  1. StartProfileRun plans the whole follow tree from the relationships and
//     the stored profiles (breadth-first, in a fixed order, each (element,
//     profile) at most once, within the follow limit), then opens the first
//     session.
//  2. Run runs the plan: each (element, profile) in its own session, its
//     steps one after another with $name variables, as jp attest does; it
//     records the variables and its part of the plan in the session, closes
//     it, then runs its children — one after another, or with Parallel at
//     most maxParallelRuns at once. Both run the same plan, so Parallel
//     changes only how long it takes.
//
// Variables belong to one run of one (element, profile): nothing passes along
// follow. Jane reports per-rule results and makes no verdict.
//
// Every session of a request is in the running set from when it opens until
// the whole request has finished, for the session page's refresh (#137).

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"a10/logging"
	"a10/structures"
)

// DefaultFollowLimit is the follow limit when none is given (decision 15).
const DefaultFollowLimit = 10

// maxParallelRuns bounds the runs going at once in a parallel request (#133).
const maxParallelRuns = 4

// RunContext says who asked for a run, through what and from where, for the
// session message (decision 35). It is filled by the REST or Web UI handler,
// so that operations never see the web framework.
type RunContext struct {
	User string // e.g. "ian", "anonymous"
	Via  string // "REST" or "Web UI"
	From string // client address
}

// RunOptions are the per-request options.
type RunOptions struct {
	FollowLimit int  // 0: don't follow
	Parallel    bool // run followed profiles at the same time (#133)
}

// ProfileRunReport is the outcome of one run of one (element, profile), with
// the runs it followed. It has no verdict.
type ProfileRunReport struct {
	ElementID   string                       `json:"elementid"`
	ElementName string                       `json:"elementname"`
	Profile     string                       `json:"profile"`
	Edited      bool                         `json:"edited,omitempty"`
	SessionID   string                       `json:"sessionid"`
	Parent      string                       `json:"parent"`
	Error       string                       `json:"error,omitempty"` // the run could not be made (e.g. its session could not be opened)
	Steps       []ProfileStepReport          `json:"steps"`
	Vars        []structures.ProfileVariable `json:"vars"`
	Follow      []ProfileFollowReport        `json:"follow"`
}

// ProfileStepReport is one attested intent and its rules.
type ProfileStepReport struct {
	Intent   string              `json:"intent"`
	Endpoint string              `json:"endpoint"`
	ClaimID  string              `json:"claimid"`
	Error    string              `json:"error"`
	Rules    []ProfileRuleReport `json:"rules"`
}

// ProfileRuleReport is one rule: run (ResultID, Result, Message), not run
// (NotRun says why) or failed to run (Error).
type ProfileRuleReport struct {
	Rule     string                  `json:"rule"`
	ResultID string                  `json:"resultid"`
	Result   *structures.ResultValue `json:"result"`
	Message  string                  `json:"message"`
	NotRun   string                  `json:"notrun"`
	Error    string                  `json:"error"`
}

// ProfileFollowReport is one follow decision and, for kind run, the report of
// the run it led to.
type ProfileFollowReport struct {
	structures.ProfileFollowEntry
	Report *ProfileRunReport `json:"report,omitempty"`
}

// ResultName names a result code, e.g. "9001 Fail", as jp does.
func ResultName(v structures.ResultValue) string {
	names := map[structures.ResultValue]string{
		structures.Success:                 "Success",
		structures.Fail:                    "Fail",
		structures.VerifyCallFailure:       "VerifyCallFailure",
		structures.VerifyClaimErrorAttempt: "VerifyClaimErrorAttempt",
		structures.NoResult:                "NoResult",
		structures.MissingExpectedValue:    "MissingExpectedValue",
		structures.RuleCallFailure:         "RuleCallFailure",
		structures.UnsetResultValue:        "UnsetResultValue",
	}
	if n, ok := names[v]; ok {
		return fmt.Sprintf("%d %s", v, n)
	}
	return fmt.Sprintf("%d unknown result", v)
}

// --- the running set (#137) --------------------------------------------------

var (
	runningMu sync.Mutex
	running   = map[string]bool{}
)

// IsRunning reports whether the session belongs to a profile request that
// has not finished yet. After a restart nothing is running.
func IsRunning(sid string) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	return running[sid]
}

func setRunning(sids []string, on bool) {
	runningMu.Lock()
	defer runningMu.Unlock()
	for _, sid := range sids {
		if on {
			running[sid] = true
		} else {
			delete(running, sid)
		}
	}
}

// --- planning ----------------------------------------------------------------

// planNode is one planned run of (element, profile).
type planNode struct {
	Element      structures.Element
	Name         string // the profile's name
	Profile      structures.AttestationProfile
	Edited       bool
	Depth        int
	Relationship string // for a followed run: the relationship it was reached by
	Follow       []structures.ProfileFollowEntry
	Children     []*planNode // aligned with the Follow entries of kind run, in order
}

// elementLookup finds an element by item ID; a variable so tests can plan
// without a database.
type elementLookup func(eid string) (structures.Element, error)

// planFollow plans the follow tree below root, breadth-first. Follow entries
// are taken in profile order and relationships by target item ID, so the
// plan, and which parent runs a shared (element, profile), never depend on
// timing. root is in the visited set from the start.
func planFollow(root *planNode, limit int, lookup elementLookup) {
	visited := map[string]bool{root.Element.ItemID + "\x00" + root.Name: true}
	queue := []*planNode{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, f := range n.Profile.Follow {
			var targets []string
			for _, r := range n.Element.Relationships {
				if r.RelationshipType == f.Relationship {
					targets = append(targets, r.ItemID)
				}
			}
			sort.Strings(targets)
			entry := structures.ProfileFollowEntry{Relationship: f.Relationship, Profile: f.Profile}
			if len(targets) == 0 {
				entry.Kind, entry.Message = structures.FollowError, "element has no "+f.Relationship+" relationship"
				n.Follow = append(n.Follow, entry)
				continue
			}
			for _, t := range targets {
				e := entry
				e.Target = t
				if n.Depth+1 > limit {
					e.Kind, e.Message = structures.FollowLimit, fmt.Sprintf("follow limit %d reached", limit)
					n.Follow = append(n.Follow, e)
					continue
				}
				te, err := lookup(t)
				if err != nil {
					e.Kind, e.Message = structures.FollowError, "related element "+t+" is missing"
					n.Follow = append(n.Follow, e)
					continue
				}
				name, p, err := ElementProfile(te, f.Profile)
				if err != nil {
					e.Kind, e.Message = structures.FollowError, fmt.Sprintf("element %s (%s): %v", te.Name, t, err)
					n.Follow = append(n.Follow, e)
					continue
				}
				e.Profile = name // the profile this follow resolved to
				key := t + "\x00" + name
				if visited[key] {
					e.Kind, e.Message = structures.FollowSkipped, "profile "+name+" of this element is already in the plan for this request"
					n.Follow = append(n.Follow, e)
					continue
				}
				visited[key] = true
				e.Kind = structures.FollowRun
				n.Follow = append(n.Follow, e)
				child := &planNode{Element: te, Name: name, Profile: p, Depth: n.Depth + 1, Relationship: f.Relationship}
				n.Children = append(n.Children, child)
				queue = append(queue, child)
			}
		}
	}
}

// --- running -----------------------------------------------------------------

// ProfileRequest is a planned profile run whose first session is open.
type ProfileRequest struct {
	SessionID string // the first run's session
	root      *planNode
	opts      RunOptions
	rc        RunContext
	parent    string

	mu       sync.Mutex
	sessions []string // every session opened by this request, for the running set
}

// StartProfileRun plans a run of the profile p (named name; edited: an
// unsaved edit from the Web UI) on the element e and opens its session under
// parent (empty: none). Call Run next, in the request or in the background.
// Errors here mean nothing was run.
func StartProfileRun(e structures.Element, name string, p structures.AttestationProfile, edited bool, parent string, opts RunOptions, rc RunContext) (*ProfileRequest, error) {
	if opts.FollowLimit < 0 {
		return nil, errors.New("the follow limit must not be negative")
	}
	root := &planNode{Element: e, Name: name, Profile: p, Edited: edited}
	planFollow(root, opts.FollowLimit, GetElementByItemID)

	req := &ProfileRequest{root: root, opts: opts, rc: rc, parent: parent}
	ed := ""
	if edited {
		ed = " (edited)"
	}
	msg := fmt.Sprintf("runprov %s%s on %s (%s) by %s via %s from %s; followlimit %d", name, ed, e.Name, e.ItemID, rc.User, rc.Via, rc.From, opts.FollowLimit)
	if opts.Parallel {
		msg += "; parallel"
	}
	sid, err := req.openSession(root, msg, parent)
	if err != nil {
		return nil, err
	}
	req.SessionID = sid
	logging.MakeLogEntry("S", "runprov", sid, "session", fmt.Sprintf("start %s on %s", name, e.ItemID))
	return req, nil
}

func (req *ProfileRequest) openSession(n *planNode, msg string, parent string) (string, error) {
	pr := &structures.ProfileRun{
		ElementID:   n.Element.ItemID,
		Name:        n.Name,
		Edited:      n.Edited,
		Profile:     ProfileYAML(n.Profile),
		FollowLimit: req.opts.FollowLimit,
		Depth:       n.Depth,
		Parallel:    req.opts.Parallel,
	}
	sid, err := OpenSessionWithParent(msg, parent, pr)
	if err != nil {
		return "", err
	}
	req.mu.Lock()
	req.sessions = append(req.sessions, sid)
	req.mu.Unlock()
	setRunning([]string{sid}, true)
	return sid, nil
}

// Run runs the planned request to the end and returns its report. All of the
// request's sessions leave the running set when it returns, whatever
// happens.
func (req *ProfileRequest) Run() (report *ProfileRunReport) {
	defer func() {
		req.mu.Lock()
		sids := req.sessions
		req.mu.Unlock()
		setRunning(sids, false)
		logging.MakeLogEntry("S", "runprov", req.SessionID, "session", "end")
	}()
	var sem chan struct{}
	if req.opts.Parallel {
		sem = make(chan struct{}, maxParallelRuns)
	}
	return req.runNode(req.root, req.SessionID, req.parent, sem)
}

// runOwn runs a node's own steps in its session sid, records the variables
// and follow decisions, and closes the session. A panic (e.g. in a rule) is
// the run's error: the session is still closed, the parallel slot is always
// given back, and the node's children still run.
func (req *ProfileRequest) runOwn(n *planNode, sid string, rep *ProfileRunReport, sem chan struct{}) {
	if sem != nil {
		sem <- struct{}{}
		defer func() { <-sem }()
	}
	defer func() {
		if r := recover(); r != nil {
			rep.Error = joinError(rep.Error, fmt.Sprintf("run failed: %v", r))
			logging.MakeLogEntry("S", "runprov", sid, "session", fmt.Sprintf("run panicked: %v", r))
		}
		if err := CloseSession(sid); err != nil {
			rep.Error = joinError(rep.Error, err.Error())
			logging.MakeLogEntry("S", "runprov", sid, "session", "close: "+err.Error())
		}
	}()
	rep.Steps, rep.Vars = runSteps(n.Element, n.Profile, sid)
	if err := SetSessionProfileResults(sid, rep.Vars, n.Follow); err != nil {
		rep.Error = joinError(rep.Error, err.Error())
		logging.MakeLogEntry("S", "runprov", sid, "session", err.Error())
	}
}

// runNode runs one planned (element, profile) in the session sid (opened
// already), then its children.
func (req *ProfileRequest) runNode(n *planNode, sid string, parent string, sem chan struct{}) *ProfileRunReport {
	rep := &ProfileRunReport{
		ElementID: n.Element.ItemID, ElementName: n.Element.Name, Profile: n.Name, Edited: n.Edited,
		SessionID: sid, Parent: parent,
		Steps: []ProfileStepReport{}, Vars: []structures.ProfileVariable{}, Follow: []ProfileFollowReport{},
	}

	req.runOwn(n, sid, rep, sem)

	// the children, in plan order; with sem, at the same time
	children := make([]*ProfileRunReport, len(n.Children))
	var wg sync.WaitGroup
	for i, c := range n.Children {
		run := func(i int, c *planNode) {
			msg := fmt.Sprintf("runprov %s on %s (%s), follow %s from session %s", c.Name, c.Element.Name, c.Element.ItemID, c.Relationship, sid)
			csid, err := req.openSession(c, msg, sid)
			if err != nil {
				children[i] = &ProfileRunReport{ElementID: c.Element.ItemID, ElementName: c.Element.Name, Profile: c.Name, Parent: sid,
					Error: "session could not be opened: " + err.Error(), Steps: []ProfileStepReport{}, Vars: []structures.ProfileVariable{}, Follow: []ProfileFollowReport{}}
				logging.MakeLogEntry("S", "runprov", sid, "session", "follow "+c.Element.ItemID+": "+err.Error())
				return
			}
			children[i] = req.runNode(c, csid, sid, sem)
		}
		if sem == nil {
			run(i, c)
			continue
		}
		wg.Add(1)
		go func(i int, c *planNode) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					children[i] = &ProfileRunReport{ElementID: c.Element.ItemID, ElementName: c.Element.Name, Profile: c.Name, Parent: sid,
						Error: fmt.Sprintf("run failed: %v", r), Steps: []ProfileStepReport{}, Vars: []structures.ProfileVariable{}, Follow: []ProfileFollowReport{}}
					logging.MakeLogEntry("S", "runprov", sid, "session", fmt.Sprintf("follow %s panicked: %v", c.Element.ItemID, r))
				}
			}()
			run(i, c)
		}(i, c)
	}
	wg.Wait()

	ci := 0
	for _, f := range n.Follow {
		fr := ProfileFollowReport{ProfileFollowEntry: f}
		if f.Kind == structures.FollowRun {
			fr.Report = children[ci]
			ci++
		} else if f.Kind == structures.FollowError {
			logging.MakeLogEntry("S", "runprov", sid, "session", "follow "+f.Relationship+": "+f.Message)
		}
		rep.Follow = append(rep.Follow, fr)
	}
	return rep
}

func joinError(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// runSteps runs a profile's steps in the session sid, one after another, as
// jp attest does: an intent that can't be attested is recorded and the run
// carries on; a rule whose $variable has no value (its step failed) is not
// run.
func runSteps(e structures.Element, p structures.AttestationProfile, sid string) ([]ProfileStepReport, []structures.ProfileVariable) {
	steps := []ProfileStepReport{}
	vars := []structures.ProfileVariable{}
	values := map[string]string{}
	set := func(name, kind, value string) {
		if name == "" {
			return
		}
		values[name] = value
		vars = append(vars, structures.ProfileVariable{Name: name, Kind: kind, Value: value})
	}

	for _, st := range p.Steps {
		sr := ProfileStepReport{Intent: st.Intent, Endpoint: st.Endpoint, Rules: []ProfileRuleReport{}}
		claim, err := attestStep(e, st, sid)
		if err != nil {
			sr.Error = err.Error()
			steps = append(steps, sr)
			continue
		}
		sr.ClaimID = claim.ItemID
		set(st.CID, "cid", claim.ItemID)

		for _, r := range st.Rules {
			rr := ProfileRuleReport{Rule: r.Name}
			params, unset := substituteVariables(r.Parameters, values)
			if unset != "" {
				rr.NotRun = "$" + unset + " has no value (its step failed)"
				sr.Rules = append(sr.Rules, rr)
				continue
			}
			rule, err := GetRule(r.Name)
			if err != nil {
				rr.Error = "no such rule"
				sr.Rules = append(sr.Rules, rr)
				continue
			}
			session, err := GetSessionByItemID(sid)
			if err != nil {
				rr.Error = "session: " + err.Error()
				sr.Rules = append(sr.Rules, rr)
				continue
			}
			rid, rv, err := Verify(claim, rule, session, params)
			rr.ResultID = rid
			if rid != "" {
				v := rv
				rr.Result = &v
				if res, gerr := GetResultByItemID(rid); gerr == nil {
					rr.Message = res.Message
				}
			}
			if err != nil {
				rr.Error = err.Error()
			}
			set(r.RID, "rid", rid)
			code := "no result"
			if rr.Result != nil {
				code = ResultName(*rr.Result)
			}
			set(r.ResVar, "resvar", code)
			sr.Rules = append(sr.Rules, rr)
		}
		steps = append(steps, sr)
	}
	return steps, vars
}

// attestStep attests one step's intent and returns the claim.
func attestStep(e structures.Element, st structures.ProfileStep, sid string) (structures.Claim, error) {
	intent, err := GetIntentByItemID(st.Intent)
	if err != nil {
		return structures.Claim{}, fmt.Errorf("intent %s not found", st.Intent)
	}
	session, err := GetSessionByItemID(sid)
	if err != nil {
		return structures.Claim{}, fmt.Errorf("session %s: %w", sid, err)
	}
	cid, err := Attest(e, st.Endpoint, intent, session, nil)
	if err != nil {
		return structures.Claim{}, err
	}
	return GetClaimByItemID(cid)
}

// substituteVariables returns params with every value that is exactly "$name"
// replaced by that variable, or the name of a variable that has no value.
func substituteVariables(params map[string]interface{}, values map[string]string) (map[string]interface{}, string) {
	if len(params) == 0 {
		return nil, ""
	}
	out := make(map[string]interface{}, len(params))
	for k, v := range params {
		if name, ok := variableRef(v); ok {
			val, set := values[name]
			if !set || val == "" {
				return nil, name
			}
			out[k] = val
			continue
		}
		out[k] = v
	}
	return out, ""
}
