package operations

// Attestation profiles on elements (#128). A profile is written only by
// SetAttestationProfile (REST PUT /element/:itemid/attestationprofile/:name,
// used by jp create/update on the device); UpdateElement keeps the stored
// profiles and default. Nothing here deletes a profile.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"a10/datalayer"
	"a10/logging"
	"a10/structures"

	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
)

var (
	ErrorProfileName     = errors.New("invalid attestation profile name: letters, digits, _ and -, at most 64 characters")
	ErrorProfileEmpty    = errors.New("an attestation profile needs at least one step or one follow entry")
	ErrorProfileStep     = errors.New("every step of an attestation profile needs an intent and an endpoint")
	ErrorProfileVariable = errors.New("attestation profile variables")
	ErrorProfileFollow   = errors.New("every follow entry needs a relationship, and a valid profile name if one is given")
	ErrorProfileNotFound = errors.New("no such attestation profile")
	ErrorNoDefault       = errors.New("the element has no default attestation profile")
)

var profileName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidProfileName reports whether name may name an attestation profile; such
// names are safe in URLs and as MongoDB field names.
func ValidProfileName(name string) bool {
	return profileName.MatchString(name)
}

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// variableRef returns the variable named by a parameter value that is exactly
// "$name".
func variableRef(v interface{}) (string, bool) {
	s, ok := v.(string)
	if !ok || !strings.HasPrefix(s, "$") {
		return "", false
	}
	return s[1:], true
}

// CheckProfile checks a profile as jp checks a provisioning file (#72):
// at least one step or follow entry, every step with an intent and an
// endpoint, and variables single-assignment (decision 43) — every cid, rid
// and resvar name valid and defined once (one namespace), and every $name in
// rule parameters defined earlier (a step's own cid counts for its rules).
// Intents, endpoints and rules are not looked up: a run reports them.
func CheckProfile(p structures.AttestationProfile) error {
	if len(p.Steps) == 0 && len(p.Follow) == 0 {
		return ErrorProfileEmpty
	}
	defined := map[string]bool{}
	define := func(name, what string) error {
		if name == "" {
			return nil
		}
		if !variableName.MatchString(name) {
			return fmt.Errorf("%w: %s: name %q must be letters, digits and _, not starting with a digit", ErrorProfileVariable, what, name)
		}
		if defined[name] {
			return fmt.Errorf("%w: %s: %s is defined more than once", ErrorProfileVariable, what, name)
		}
		defined[name] = true
		return nil
	}
	for i, st := range p.Steps {
		if st.Intent == "" || st.Endpoint == "" {
			return fmt.Errorf("%w (step %d)", ErrorProfileStep, i+1)
		}
		if err := define(st.CID, st.Intent+" cid"); err != nil {
			return err
		}
		for _, r := range st.Rules {
			where := st.Intent + " rule " + r.Name
			for k, v := range r.Parameters {
				if name, ok := variableRef(v); ok && !defined[name] {
					return fmt.Errorf("%w: %s: parameter %s refers to $%s, which is not defined before it", ErrorProfileVariable, where, k, name)
				}
			}
			if err := define(r.RID, where+" rid"); err != nil {
				return err
			}
			if err := define(r.ResVar, where+" resvar"); err != nil {
				return err
			}
		}
	}
	for _, f := range p.Follow {
		if f.Relationship == "" || (f.Profile != "" && !ValidProfileName(f.Profile)) {
			return fmt.Errorf("%w (follow %q, profile %q)", ErrorProfileFollow, f.Relationship, f.Profile)
		}
	}
	return nil
}

// SetAttestationProfile writes (or overwrites) the profile name of the
// element eid and makes it the element's default, in one atomic update
// (decision 2). The element's other profiles are untouched.
func SetAttestationProfile(eid string, name string, p structures.AttestationProfile) error {
	if !ValidProfileName(name) {
		return ErrorProfileName
	}
	if err := CheckProfile(p); err != nil {
		return err
	}
	filter := bson.D{{"itemid", eid}}
	update := bson.D{{"$set", bson.D{
		{"attestationprofiles." + name, p},
		{"defaultattestationprofile", name},
	}}}
	res, err := datalayer.DB.Collection("elements").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return ErrorItemNotFound
	}
	logging.MakeLogEntry("IM", "setprofile", eid, "element", name)
	return nil
}

// ElementProfile returns the profile name of the element e, or its default
// if name is empty, with the name actually used.
func ElementProfile(e structures.Element, name string) (string, structures.AttestationProfile, error) {
	if name == "" {
		name = e.DefaultAttestationProfile
		if name == "" {
			return "", structures.AttestationProfile{}, ErrorNoDefault
		}
	}
	p, ok := e.AttestationProfiles[name]
	if !ok {
		return name, structures.AttestationProfile{}, fmt.Errorf("%w: %s", ErrorProfileNotFound, name)
	}
	return name, p, nil
}

// ProfileYAML is the YAML text of a profile, as shown on the profile page and
// recorded in a profile run's session.
func ProfileYAML(p structures.AttestationProfile) string {
	b, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Sprintf("# the profile could not be written as YAML: %v\n", err)
	}
	return string(b)
}

// ParseProfileYAML reads a profile from YAML (the Web UI's edit box) and
// checks it. Unknown keys are an error, so a mistyped key isn't silently lost.
func ParseProfileYAML(text string) (structures.AttestationProfile, error) {
	var p structures.AttestationProfile
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("the profile is not valid YAML: %w", err)
	}
	if err := CheckProfile(p); err != nil {
		return p, err
	}
	return p, nil
}

// SameProfile reports whether two profiles are the same, compared as they are
// written in YAML (so a parameter read back as an int still matches).
func SameProfile(a, b structures.AttestationProfile) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	return ProfileYAML(a) == ProfileYAML(b)
}
