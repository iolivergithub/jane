package janeapi

import "regexp"

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

var profileName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidProfileName reports whether name may name an attestation profile:
// letters, digits, _ and -, at most 64 characters, so it is safe in URLs and
// as a MongoDB field name.
func ValidProfileName(name string) bool {
	return profileName.MatchString(name)
}
