package structures

import "github.com/iolivergithub/jane/lib/janeapi"

// Attestation profiles (#128): named attestation instructions stored on an
// element, written only by jp create/update on the device, run by Jane on
// request (#129). A profile is the attestation part of a provisioning file's
// evs: list, plus the elements to follow.

// AttestationProfile and its parts are shared with jp (lib/janeapi).
type AttestationProfile = janeapi.AttestationProfile

type ProfileStep = janeapi.ProfileStep

type ProfileRule = janeapi.ProfileRule

type ProfileFollow = janeapi.ProfileFollow

// ProfileRun is recorded on the session of a profile run (#130): which
// profile ran, exactly what it was, and what the run produced. ElementID,
// Name, Edited, Profile, FollowLimit and Depth are set when the session is
// opened; Variables and Follow just before it is closed. All of it is signed
// with the session.
type ProfileRun struct {
	ElementID   string               `json:"elementid" bson:"elementid"`
	Name        string               `json:"name" bson:"name"`
	Edited      bool                 `json:"edited" bson:"edited"`
	Profile     string               `json:"profile" bson:"profile"` // YAML text of the profile as run
	FollowLimit int                  `json:"followlimit" bson:"followlimit"`
	Depth       int                  `json:"depth" bson:"depth"`
	Parallel    bool                 `json:"parallel,omitempty" bson:"parallel,omitempty"`
	Variables   []ProfileVariable    `json:"variables,omitempty" bson:"variables,omitempty"`
	Follow      []ProfileFollowEntry `json:"follow,omitempty" bson:"follow,omitempty"`
}

// ProfileVariable is a variable set during a run: Kind is cid, rid or resvar.
type ProfileVariable struct {
	Name  string `json:"name" bson:"name"`
	Kind  string `json:"kind" bson:"kind"`
	Value string `json:"value" bson:"value"`
}

// ProfileFollowEntry is one follow decision: for a follow entry and one
// matching relationship, the target and profile, and what was decided.
type ProfileFollowEntry struct {
	Relationship string `json:"relationship" bson:"relationship"`
	Target       string `json:"target" bson:"target"`
	Profile      string `json:"profile" bson:"profile"` // the profile followed: once the target's profile is found, its name (also for the default); otherwise as given in the follow entry, empty meaning the default
	Kind         string `json:"kind" bson:"kind"`       // FollowRun, FollowSkipped, FollowLimit or FollowError
	Message      string `json:"message" bson:"message"`
}

// The kinds of follow decision (machine-readable; Message is for people).
const (
	FollowRun     = "run"
	FollowSkipped = "skipped"
	FollowLimit   = "limit"
	FollowError   = "error"
)
