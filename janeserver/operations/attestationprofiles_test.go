package operations

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"a10/signing"
	"a10/structures"
)

func TestValidProfileName(t *testing.T) {
	for _, n := range []string{"default", "nightly", "Host_check-2", strings.Repeat("a", 64)} {
		if !ValidProfileName(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	for _, n := range []string{"", "a.b", "$x", "a/b", "a b", strings.Repeat("a", 65)} {
		if ValidProfileName(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}

func step(intent, cid string, rules ...structures.ProfileRule) structures.ProfileStep {
	return structures.ProfileStep{Intent: intent, Endpoint: "tarzan", CID: cid, Rules: rules}
}

func TestCheckProfile(t *testing.T) {
	ok := []structures.AttestationProfile{
		{Steps: []structures.ProfileStep{step("i1", "c1", structures.ProfileRule{Name: "r", RID: "r1", ResVar: "v1", Parameters: map[string]interface{}{"x": "$c1"}}),
			step("i2", "", structures.ProfileRule{Name: "r", Parameters: map[string]interface{}{"x": "$r1", "y": "$v1"}})}},
		{Follow: []structures.ProfileFollow{{Relationship: "hosted-on"}}}, // only follow (decision 37)
	}
	for i, p := range ok {
		if err := CheckProfile(p); err != nil {
			t.Errorf("profile %d: %v", i, err)
		}
	}
	bad := map[string]struct {
		p    structures.AttestationProfile
		want error
	}{
		"empty":           {structures.AttestationProfile{}, ErrorProfileEmpty},
		"no endpoint":     {structures.AttestationProfile{Steps: []structures.ProfileStep{{Intent: "i"}}}, ErrorProfileStep},
		"defined twice":   {structures.AttestationProfile{Steps: []structures.ProfileStep{step("i1", "a"), step("i2", "a")}}, ErrorProfileVariable},
		"cid and rid":     {structures.AttestationProfile{Steps: []structures.ProfileStep{step("i1", "a", structures.ProfileRule{Name: "r", RID: "a"})}}, ErrorProfileVariable},
		"used before":     {structures.AttestationProfile{Steps: []structures.ProfileStep{step("i1", "", structures.ProfileRule{Name: "r", Parameters: map[string]interface{}{"x": "$later"}}), step("i2", "later")}}, ErrorProfileVariable},
		"bad name":        {structures.AttestationProfile{Steps: []structures.ProfileStep{step("i1", "1a")}}, ErrorProfileVariable},
		"follow no type":  {structures.AttestationProfile{Follow: []structures.ProfileFollow{{}}}, ErrorProfileFollow},
		"follow bad name": {structures.AttestationProfile{Follow: []structures.ProfileFollow{{Relationship: "runs-on", Profile: "a.b"}}}, ErrorProfileFollow},
	}
	for name, c := range bad {
		if err := CheckProfile(c.p); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestProfileYAMLRoundTrip(t *testing.T) {
	p := structures.AttestationProfile{
		Description: "nightly check",
		Steps: []structures.ProfileStep{step("std::intent::sha256::crtm::pcr0", "q",
			structures.ProfileRule{Name: "tpm2_attestedValue"},
			structures.ProfileRule{Name: "tpm2_clockProgress", Parameters: map[string]interface{}{"previouscid": "$q"}})},
		Follow: []structures.ProfileFollow{{Relationship: "hosted-on", Profile: "hostcheck"}},
	}
	text := ProfileYAML(p)
	got, err := ParseProfileYAML(text)
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	if !SameProfile(p, got) {
		t.Errorf("round trip changed the profile:\n%s\n%s", text, ProfileYAML(got))
	}
	edited := strings.Replace(text, "tpm2_attestedValue", "tpm2_validSignature", 1)
	e, err := ParseProfileYAML(edited)
	if err != nil || SameProfile(p, e) {
		t.Errorf("an edit is not noticed: %v", err)
	}
	if _, err := ParseProfileYAML("steps:\n  - intent: i\n    endpont: tarzan\n"); err == nil {
		t.Error("a mistyped key is accepted")
	}
}

// The substitution rules are shared with jp: the same fixture is used by
// provisioner's tests.
func TestSubstituteVariablesFixture(t *testing.T) {
	var fx struct {
		Cases []struct {
			Name   string                 `json:"name"`
			Params map[string]interface{} `json:"params"`
			Vars   map[string]string      `json:"vars"`
			Want   map[string]interface{} `json:"want"`
			Unset  string                 `json:"unset"`
		} `json:"cases"`
	}
	b, err := os.ReadFile("testdata/profilevariables.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatal(err)
	}
	for _, c := range fx.Cases {
		got, unset := substituteVariables(c.Params, c.Vars)
		if unset != c.Unset || !reflect.DeepEqual(got, c.Want) {
			t.Errorf("%s: got %v %q, want %v %q", c.Name, got, unset, c.Want, c.Unset)
		}
	}
}

// fakeElements is a lookup over a fixed set of elements.
type fakeElements map[string]structures.Element

func (f fakeElements) get(eid string) (structures.Element, error) {
	e, ok := f[eid]
	if !ok {
		return e, ErrorItemNotFound
	}
	return e, nil
}

func el(id string, rels map[string][]string, profiles map[string]structures.AttestationProfile, def string) structures.Element {
	e := structures.Element{ItemID: id, Name: id, AttestationProfiles: profiles, DefaultAttestationProfile: def}
	for typ, targets := range rels {
		for _, t := range targets {
			e.Relationships = append(e.Relationships, structures.ElementRelationship{RelationshipType: typ, ItemID: t})
		}
	}
	return e
}

func follows(rel ...string) structures.AttestationProfile {
	p := structures.AttestationProfile{Steps: []structures.ProfileStep{step("i", "")}}
	for _, r := range rel {
		p.Follow = append(p.Follow, structures.ProfileFollow{Relationship: r})
	}
	return p
}

// plan returns "element/profile@depth" for every planned run, breadth-first,
// and the kinds of every follow decision in the same order.
func plan(root *planNode) (runs []string, kinds []string) {
	queue := []*planNode{root}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		runs = append(runs, n.Element.ItemID+"/"+n.Name+"@"+string(rune('0'+n.Depth)))
		for _, f := range n.Follow {
			kinds = append(kinds, n.Element.ItemID+">"+f.Target+":"+f.Kind)
		}
		queue = append(queue, n.Children...)
	}
	return
}

func TestPlanFollow(t *testing.T) {
	p := follows("hosted-on")
	// diamond A->B, A->C, B->D, C->D; D also back to A (a cycle)
	els := fakeElements{
		"A": el("A", map[string][]string{"hosted-on": {"C", "B"}}, map[string]structures.AttestationProfile{"p": p}, "p"),
		"B": el("B", map[string][]string{"hosted-on": {"D"}}, map[string]structures.AttestationProfile{"p": p}, "p"),
		"C": el("C", map[string][]string{"hosted-on": {"D"}}, map[string]structures.AttestationProfile{"p": p}, "p"),
		"D": el("D", map[string][]string{"hosted-on": {"A"}}, map[string]structures.AttestationProfile{"p": p}, "p"),
	}
	root := &planNode{Element: els["A"], Name: "p", Profile: p}
	planFollow(root, DefaultFollowLimit, els.get)
	runs, kinds := plan(root)
	wantRuns := []string{"A/p@0", "B/p@1", "C/p@1", "D/p@2"}
	wantKinds := []string{"A>B:run", "A>C:run", "B>D:run", "C>D:skipped", "D>A:skipped"}
	if !reflect.DeepEqual(runs, wantRuns) || !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("diamond and cycle:\nruns  %v\nwant  %v\nkinds %v\nwant  %v", runs, wantRuns, kinds, wantKinds)
	}

	// the limit
	root = &planNode{Element: els["A"], Name: "p", Profile: p}
	planFollow(root, 1, els.get)
	_, kinds = plan(root)
	wantKinds = []string{"A>B:run", "A>C:run", "B>D:limit", "C>D:limit"}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("limit 1: %v, want %v", kinds, wantKinds)
	}
	root = &planNode{Element: els["A"], Name: "p", Profile: p}
	planFollow(root, 0, els.get)
	if _, kinds = plan(root); !reflect.DeepEqual(kinds, []string{"A>B:limit", "A>C:limit"}) {
		t.Errorf("limit 0: %v", kinds)
	}

	// errors: no relationship of the type, a missing element, no such profile, no default
	q := structures.AttestationProfile{Follow: []structures.ProfileFollow{{Relationship: "runs-on"}, {Relationship: "hosted-on", Profile: "nope"}, {Relationship: "hosted-on"}}}
	els2 := fakeElements{
		"X": el("X", map[string][]string{"hosted-on": {"Y", "Z"}}, nil, ""),
		"Y": el("Y", nil, map[string]structures.AttestationProfile{"p": p}, ""),
	}
	root = &planNode{Element: els2["X"], Name: "q", Profile: q}
	planFollow(root, DefaultFollowLimit, els2.get)
	_, kinds = plan(root)
	wantKinds = []string{"X>:error", "X>Y:error", "X>Z:error", "X>Y:error", "X>Z:error"}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("errors: %v, want %v", kinds, wantKinds)
	}
	for _, f := range root.Follow {
		if f.Message == "" {
			t.Errorf("follow %+v has no message", f)
		}
	}
}

// Sessions without a parent or profile run sign exactly the same bytes as
// before 1.6 (#130).
func TestSessionHashUnchanged(t *testing.T) {
	type before struct {
		ItemID     string
		Timing     structures.SessionTiming
		ClaimList  []string
		ResultList []string
		Message    string
	}
	b := before{"s1", structures.SessionTiming{Opened: 1, Closed: 2}, []string{"c"}, []string{"r"}, "m"}
	n := hashablePartSession{"s1", structures.SessionTiming{Opened: 1, Closed: 2}, []string{"c"}, []string{"r"}, "m", "", nil}
	cb, err1 := signing.Canonicalise(b)
	cn, err2 := signing.Canonicalise(n)
	if err1 != nil || err2 != nil || string(cb) != string(cn) {
		t.Errorf("hashable part changed:\n%s\n%s", cb, cn)
	}
	n.Parent = "p"
	if cp, _ := signing.Canonicalise(n); string(cp) == string(cb) {
		t.Error("the parent is not signed")
	}
}

func TestRunningSet(t *testing.T) {
	setRunning([]string{"a", "b"}, true)
	if !IsRunning("a") || !IsRunning("b") || IsRunning("c") {
		t.Error("running set after adding")
	}
	setRunning([]string{"a", "b"}, false)
	if IsRunning("a") || IsRunning("b") {
		t.Error("running set after removing")
	}
}
