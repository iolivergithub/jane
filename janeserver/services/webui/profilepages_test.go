package webui

import (
	"bytes"
	"strings"
	"testing"

	"a10/structures"
)

// renderPage renders a whole page (with base.html) as the Web UI does.
func renderPage(t *testing.T, name string, data interface{}) string {
	t.Helper()
	page, ok := parseTemplates(templateFunctions())[name]
	if !ok {
		t.Fatalf("no page %s", name)
	}
	var out bytes.Buffer
	if err := page.ExecuteTemplate(&out, "base.html", data); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func profileElement() structures.Element {
	return structures.Element{ItemID: "E1", Name: "web01", DefaultAttestationProfile: "nightly",
		AttestationProfiles: map[string]structures.AttestationProfile{
			"nightly": {Description: "Nightly check", Steps: []structures.ProfileStep{{Intent: "std::intent::sys::info", Endpoint: "tarzan",
				Rules: []structures.ProfileRule{{Name: "sys_taRunningSafely"}}}}, Follow: []structures.ProfileFollow{{Relationship: "hosted-on"}}},
			"quick": {Steps: []structures.ProfileStep{{Intent: "std::intent::sys::info", Endpoint: "tarzan"}}},
		}}
}

func TestElementProfilesTab(t *testing.T) {
	html := renderPage(t, "element.html", elementPage{E: profileElement()})
	for _, want := range []string{"Profiles", `/element/E1/profile/nightly`, `/element/E1/profile/quick`, "Nightly check", ">default<", "hosted-on"} {
		if !strings.Contains(html, want) {
			t.Errorf("element page lacks %q", want)
		}
	}
	if strings.Index(html, "profile/nightly") > strings.Index(html, "profile/quick") {
		t.Error("profiles not sorted by name")
	}
}

func TestProfilePage(t *testing.T) {
	page := newProfilePage(profileElement(), "nightly")
	html := renderPage(t, "profile.html", page)
	for _, want := range []string{"Profile nightly", "default", "readonly", "sys_taRunningSafely", `name="follow"`, `name="depth"`, `name="parallel"`, `value="attest"`, "?edit=1"} {
		if !strings.Contains(html, want) {
			t.Errorf("profile page lacks %q", want)
		}
	}
	page.Editable = true
	if html := renderPage(t, "profile.html", page); strings.Contains(html, "readonly") || !strings.Contains(html, "never saved") {
		t.Error("an editable page is read-only or lacks the warning")
	}
	page.Error = "the profile is not valid YAML"
	if html := renderPage(t, "profile.html", page); !strings.Contains(html, "nothing was run") {
		t.Error("error not shown")
	}
	if html := renderPage(t, "profile.html", newProfilePage(profileElement(), "nope")); !strings.Contains(html, "not found") {
		t.Error("missing profile not reported")
	}
}

func runSession(running bool) (structures.Session, []structures.Session, sessionTree) {
	pr := &structures.ProfileRun{ElementID: "E1", Name: "nightly", Edited: true, Profile: "steps: []\n", FollowLimit: 10,
		Variables: []structures.ProfileVariable{{Name: "q", Kind: "cid", Value: "C1"}, {Name: "r", Kind: "rid", Value: "R1"}},
		Follow: []structures.ProfileFollowEntry{
			{Relationship: "hosted-on", Target: "H1", Profile: "default", Kind: structures.FollowRun},
			{Relationship: "hosted-on", Target: "H2", Profile: "default", Kind: structures.FollowRun},
			{Relationship: "hosted-on", Target: "H3", Kind: structures.FollowSkipped, Message: "already in the plan"},
			{Relationship: "runs-on", Kind: structures.FollowError, Message: "element has no runs-on relationship"},
		}}
	s := structures.Session{ItemID: "S1", Parent: "S0", Message: "runprov nightly (edited)", AttestationProfileRun: pr,
		Timing: structures.SessionTiming{Opened: 1, Closed: 2}}
	children := []structures.Session{{ItemID: "S2", Parent: "S1", AttestationProfileRun: &structures.ProfileRun{ElementID: "H1", Name: "default"}}}
	els := map[string]structures.Element{"E1": profileElement(), "H1": {ItemID: "H1", Name: "host1"}}
	lk := &lookups{Element: func(id string) (structures.Element, bool) { e, ok := els[id]; return e, ok }}
	return s, children, buildSessionTree(s, false, children, running, 2, lk)
}

func TestSessionTree(t *testing.T) {
	_, _, tr := runSession(false)
	if !tr.ParentMissing || tr.Refresh != 0 || tr.Run == nil {
		t.Fatalf("tree %+v", tr)
	}
	v := tr.Run
	if v.NRun != 2 || v.NSkipped != 1 || v.NError != 1 || !v.ProfileFound || !v.StoredChanged {
		t.Errorf("view %+v", v)
	}
	if v.Follow[0].ChildSession != "S2" || v.Follow[0].TargetName != "host1" {
		t.Errorf("run row %+v", v.Follow[0])
	}
	if !v.Follow[1].NotStarted {
		t.Error("a run entry without a session, request finished: not reported as not started")
	}
	_, _, tr = runSession(true)
	if tr.Refresh != 2 || tr.Run.Follow[1].NotStarted {
		t.Errorf("while running: refresh %d, not started %v", tr.Refresh, tr.Run.Follow[1].NotStarted)
	}
}

func TestSessionPageProfileRun(t *testing.T) {
	s, _, tr := runSession(true)
	html := renderPage(t, "session.html", sessionPage{S: s, Tree: tr})
	for _, want := range []string{`http-equiv="refresh" content="2"`, "Parent session", "(missing)", "Profile run:", "nightly", "edited",
		"2 variables", "follow 4 (2 run, 1 skipped, 0 limit, 1 errors)", `href="/claim/C1"`, `href="/result/R1"`,
		`href="/session/S2"`, "Sessions with this as parent", "Edit again", "the stored profile has changed", "<details class=\"card mb-3\" open"} {
		if !strings.Contains(html, want) {
			t.Errorf("session page lacks %q", want)
		}
	}
	s, _, tr = runSession(false)
	html = renderPage(t, "session.html", sessionPage{S: s, Tree: tr})
	if strings.Contains(html, "http-equiv") || !strings.Contains(html, "follow run not started") {
		t.Error("a finished run's page still refreshes, or lacks 'follow run not started'")
	}
	// an ordinary session: none of it
	html = renderPage(t, "session.html", sessionPage{S: structures.Session{ItemID: "S9"}})
	for _, no := range []string{"http-equiv", "Profile run:", "Parent session", "Sessions with this as parent"} {
		if strings.Contains(html, no) {
			t.Errorf("ordinary session page has %q", no)
		}
	}
}
