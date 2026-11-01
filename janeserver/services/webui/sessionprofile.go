package webui

// The session page's parts for parent sessions and profile runs (#130, #132,
// #137): the parent, the sessions opened under this one, the record of a
// profile run, and whether the page should reload itself.

import (
	"a10/operations"
	"a10/structures"
)

// sessionTree is what the session page shows about a session's place in a
// tree of sessions and, for a profile run, what the run did.
type sessionTree struct {
	Parent        string
	ParentMissing bool
	Children      []childSession
	Run           *profileRunView
	Refresh       int  // seconds; 0: the page doesn't reload itself
	Running       bool // the session's request is still running
}

type childSession struct {
	S           structures.Session
	ElementName string
}

type profileRunView struct {
	PR                             *structures.ProfileRun
	ElementName                    string
	ElementFound                   bool
	ProfileFound                   bool // the element still has a profile of this name
	StoredChanged                  bool // ...and it differs from the one that ran
	Vars                           []structures.ProfileVariable
	Follow                         []followRow
	NRun, NSkipped, NLimit, NError int
}

type followRow struct {
	E            structures.ProfileFollowEntry
	TargetName   string
	ChildSession string // the session the run went into, if found
	NotStarted   bool   // kind run, no child session, and the request has finished
}

// buildSessionTree assembles the session page's tree parts. children are the
// sessions opened under s; running says whether s's request is still going;
// refresh is sessions: refresh.
func buildSessionTree(s structures.Session, parentFound bool, children []structures.Session, running bool, refresh int, lk *lookups) sessionTree {
	t := sessionTree{Parent: s.Parent, ParentMissing: s.Parent != "" && !parentFound, Running: running}
	if running {
		t.Refresh = refresh
	}
	for _, c := range children {
		cs := childSession{S: c}
		if c.AttestationProfileRun != nil {
			cs.ElementName = lk.element(c.AttestationProfileRun.ElementID).Name
		}
		t.Children = append(t.Children, cs)
	}
	pr := s.AttestationProfileRun
	if pr == nil {
		return t
	}
	v := &profileRunView{PR: pr, Vars: pr.Variables}
	if e := lk.element(pr.ElementID); e.ItemID != "" {
		v.ElementName, v.ElementFound = e.Name, true
		if p, ok := e.AttestationProfiles[pr.Name]; ok {
			v.ProfileFound = true
			v.StoredChanged = operations.ProfileYAML(p) != pr.Profile
		}
	}
	// a run entry's child: the first unused child session for that (element, profile)
	used := map[string]bool{}
	for _, f := range pr.Follow {
		row := followRow{E: f}
		row.TargetName = lk.element(f.Target).Name
		switch f.Kind {
		case structures.FollowRun:
			v.NRun++
			for _, c := range children {
				cpr := c.AttestationProfileRun
				if cpr != nil && cpr.ElementID == f.Target && !used[c.ItemID] && (f.Profile == "" || f.Profile == cpr.Name) {
					row.ChildSession = c.ItemID
					used[c.ItemID] = true
					break
				}
			}
			row.NotStarted = row.ChildSession == "" && !running
		case structures.FollowSkipped:
			v.NSkipped++
		case structures.FollowLimit:
			v.NLimit++
		default:
			v.NError++
		}
		v.Follow = append(v.Follow, row)
	}
	t.Run = v
	return t
}
