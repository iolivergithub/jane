package structures

// Attestation profiles (#128): named attestation instructions stored on an
// element, written only by jp create/update on the device, run by Jane on
// request (#129). A profile is the attestation part of a provisioning file's
// evs: list, plus the elements to follow.

// AttestationProfile is one named profile of an element.
type AttestationProfile struct {
	Description string          `json:"description,omitempty" bson:"description,omitempty" yaml:"description,omitempty"`
	Steps       []ProfileStep   `json:"steps,omitempty" bson:"steps,omitempty" yaml:"steps,omitempty"`
	Follow      []ProfileFollow `json:"follow,omitempty" bson:"follow,omitempty" yaml:"follow,omitempty"`
}

// ProfileStep attests one intent through one endpoint and runs its rules.
// CID optionally names a variable that receives the claim's item ID.
type ProfileStep struct {
	Intent   string        `json:"intent" bson:"intent" yaml:"intent"`
	Endpoint string        `json:"endpoint" bson:"endpoint" yaml:"endpoint"`
	CID      string        `json:"cid,omitempty" bson:"cid,omitempty" yaml:"cid,omitempty"`
	Rules    []ProfileRule `json:"rules,omitempty" bson:"rules,omitempty" yaml:"rules,omitempty"`
}

// ProfileRule is one rule of a step. RID and ResVar optionally name variables
// that receive the result's item ID and its code. A parameter value that is
// exactly "$name" is replaced by that variable when the rule runs.
type ProfileRule struct {
	Name       string                 `json:"name" bson:"name" yaml:"name"`
	RID        string                 `json:"rid,omitempty" bson:"rid,omitempty" yaml:"rid,omitempty"`
	ResVar     string                 `json:"resvar,omitempty" bson:"resvar,omitempty" yaml:"resvar,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty" bson:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// ProfileFollow says: after this element, run Profile (empty: the target's
// default) on every element this one has a Relationship of this type to.
type ProfileFollow struct {
	Relationship string `json:"relationship" bson:"relationship" yaml:"relationship"`
	Profile      string `json:"profile,omitempty" bson:"profile,omitempty" yaml:"profile,omitempty"`
}

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
	Profile      string `json:"profile" bson:"profile"` // as given in the follow entry; empty: the target's default
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
