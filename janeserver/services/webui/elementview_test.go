package webui

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"testing"
	"time"

	"a10/structures"
)

var base = time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC).UnixNano()

func ts(min int) structures.Timestamp { return structures.Timestamp(base + int64(min)*int64(time.Minute)) }

// fixture: nSessions sessions, 20 minutes apart; returns claims & results newest first
func fixture(nSessions int) (structures.Element, []structures.Claim, []structures.Result, map[string]structures.Session) {
	e := structures.Element{ItemID: "e-1", Name: "edge-gw-01", Description: "Edge gateway in rack 4",
		Tags: []string{"lab", "x86"}, Endpoints: map[string]structures.Endpoint{
			"tarzan": {Endpoint: "http://10.0.0.21:8530", Protocol: "A10HTTPRESTv2"},
			"ratsd":  {Endpoint: "http://10.0.0.21:8853", Protocol: "RATSD"}},
		Host: structures.HostMachine{OS: "linux", Arch: "amd64", Hostname: "edge-gw-01", MachineID: "4f1c9a0b"},
	}
	e.TPM2.Device = "/dev/tpmrm0"
	e.TPM2.EKCertHandle = "0x01c00002"
	e.TPM2.EK.Handle, e.TPM2.AK.Handle = "0x810100EE", "0x810100AA"
	e.UEFI.Eventlog = "/sys/kernel/security/tpm0/binary_bios_measurements"
	e.IMA.ASCIILog = "/sys/kernel/security/ima/ascii_runtime_measurements"
	e.RecordHistory.Created = ts(-5000)

	intents := []string{"tpm2/quote", "uefi/eventlog", "ima/asciilog", "sys/info"}
	rules := []string{"tpm2_attestedValue", "tpm2_firmware", "tpm2_magicNumber", "tpm2_safe", "uefi_eventlog_hash", "ima_policy"}

	sessions := map[string]structures.Session{}
	var cs []structures.Claim
	var rs []structures.Result
	for s := nSessions - 1; s >= 0; s-- { // newest first
		sid := fmt.Sprintf("sess-%02d", s)
		open := ts(s * 20)
		sess := structures.Session{ItemID: sid, Timing: structures.SessionTiming{Opened: open, Closed: open + structures.Timestamp(1340*time.Millisecond)},
			Message: "Invocation from Jane WebUI"}
		if s == nSessions-1 {
			sess.Timing.Closed = 0 // still open
		}
		if s != 3 { // session 3 record missing
			sessions[sid] = sess
		}
		for i, in := range intents {
			c := structures.Claim{ItemID: fmt.Sprintf("c-%d-%d", s, i), BodyType: in}
			c.Header.Intent.Name = in
			c.Header.EndpointName = "tarzan"
			c.Header.Session.ItemID = sid
			c.Header.Timing.Requested = open + structures.Timestamp(i*100)
			if in == "ima/asciilog" && s%5 == 2 {
				c.BodyType = structures.CLAIMERROR
			}
			cs = append(cs, c)
		}
		for i, rn := range rules {
			v := structures.Success
			switch {
			case rn == "ima_policy" && s >= nSessions-3:
				v = structures.Fail
			case rn == "ima_policy" && s%5 == 2:
				v = structures.ResultValue(structures.VerifyClaimErrorAttempt)
			case rn == "tpm2_firmware" && s == 6:
				v = structures.Fail
			}
			if rn == "uefi_eventlog_hash" && s < 4 {
				continue // rule added later
			}
			rs = append(rs, structures.Result{ItemID: fmt.Sprintf("r-%d-%d", s, i), RuleName: rn, ClaimID: fmt.Sprintf("c-%d-0", s),
				Session: structures.Session{ItemID: sid}, VerifiedAt: open + structures.Timestamp(time.Second), Result: v,
				Message: map[bool]string{true: "", false: "unexpected value"}[v == structures.Success]})
		}
	}
	// claims/results are newest first by construction except order inside a session; sort like the DB would
	return e, cs, rs, sessions
}

func lookupIn(m map[string]structures.Session) func(string) (structures.Session, bool) {
	return func(id string) (structures.Session, bool) { s, ok := m[id]; return s, ok }
}

func TestBuildElementPage(t *testing.T) {
	e, cs, rs, sess := fixture(40)
	p := buildElementPage(e, cs, rs, lookupIn(sess))

	if len(p.Sessions) != 40 || len(p.Timeline) != elementTimelineSessions {
		t.Fatalf("sessions=%d timeline=%d", len(p.Sessions), len(p.Timeline))
	}
	if p.Sessions[0].ID != "sess-39" || p.Latest.ID != "sess-39" {
		t.Fatalf("newest first broken: %s", p.Sessions[0].ID)
	}
	if p.Timeline[0].ID != "sess-10" || p.Timeline[len(p.Timeline)-1].ID != "sess-39" {
		t.Fatalf("timeline order: %s .. %s", p.Timeline[0].ID, p.Timeline[len(p.Timeline)-1].ID)
	}
	if p.Latest.Outcome() != outcomeFail || p.Latest.Duration != "" || !p.Latest.Found {
		t.Fatalf("latest: %+v", p.Latest.Outcome())
	}
	if d := p.Sessions[1].Duration; d != "1.34s" {
		t.Fatalf("duration %q", d)
	}
	for _, s := range p.Sessions {
		if s.ID == "sess-03" && s.Found {
			t.Fatal("missing record reported as found")
		}
		if s.ID == "sess-03" && s.Opened != ts(60) {
			t.Fatalf("fallback opened time wrong: %v", s.Opened)
		}
	}
	if len(p.RuleRows) != 6 || len(p.ClaimRows) != 4 {
		t.Fatalf("rows rules=%d claims=%d", len(p.RuleRows), len(p.ClaimRows))
	}
	for _, r := range p.RuleRows {
		if len(r.Cells) != len(p.Timeline) {
			t.Fatal("ragged matrix")
		}
		if r.Label == "ima_policy" && r.Cells[len(r.Cells)-1].Outcome != outcomeFail {
			t.Fatal("ima_policy latest should fail")
		}
	}
	if got := len(p.Chart.Labels); got != 30 || len(p.Chart.Pass) != 30 || len(p.Chart.SessionIDs) != 30 {
		t.Fatal("chart series lengths")
	}
	if p.NPass+p.NFail+p.NOther != len(rs) {
		t.Fatal("totals")
	}

	empty := buildElementPage(e, nil, nil, lookupIn(sess))
	if len(empty.Sessions) != 0 || empty.Latest != nil || empty.Timeline != nil {
		t.Fatal("empty element")
	}
	nolookup := buildElementPage(e, cs[:8], rs[:12], nil)
	if len(nolookup.Sessions) != 2 {
		t.Fatalf("nil lookup sessions=%d", len(nolookup.Sessions))
	}
}

func TestRenderElementPage(t *testing.T) {
	T := "templates/"
	fm := template.FuncMap{
		"epochToUTC":         func(e structures.Timestamp) string { return time.Unix(0, int64(e)).UTC().Format("2006-01-02 15:04:05") },
		"epochToUTCdetailed": func(e structures.Timestamp) string { return time.Unix(0, int64(e)).UTC().Format("2006-01-02 15:04:05.0000000") },
	}
	tmpl := template.Must(template.New("element.html").Funcs(fm).ParseFiles(T+"element.html", T+"base.html",
		T+"uefi.html", T+"txt.html", T+"ima.html", T+"tpm2.html", T+"tpm2key.html",
		T+"hostinformation.html", T+"recordhistory.html", T+"resultvalue.html"))

	e, cs, rs, sess := fixture(40)
	for name, p := range map[string]elementPage{
		"full":  buildElementPage(e, cs, rs, lookupIn(sess)),
		"empty": buildElementPage(structures.Element{ItemID: "e-2", Name: "fresh-node"}, nil, nil, nil),
	} {
		var b strings.Builder
		if err := tmpl.ExecuteTemplate(&b, "base.html", p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out := os.Getenv("RENDER_DIR")
		if out != "" {
			os.WriteFile(out+"/element-"+name+".html", []byte(b.String()), 0o644)
		}
		if name == "full" && (!strings.Contains(b.String(), `"sessionIds":["sess-10"`) || strings.Contains(b.String(), "ZgotmplZ")) {
			t.Fatalf("chart json or unsafe value in output")
		}
	}
}

func TestOutcome(t *testing.T) {
	cases := []struct {
		s    elementSession
		want string
	}{
		{elementSession{Pass: 3}, outcomePass},
		{elementSession{Pass: 3, Fail: 1}, outcomeFail},
		{elementSession{Pass: 3, Other: 1}, outcomeOther},
		{elementSession{Pass: 3, ClaimErrors: 1}, outcomeOther},
		{elementSession{Fail: 1, ClaimErrors: 1}, outcomeFail},
		{elementSession{}, ""},
	}
	for _, c := range cases {
		if got := c.s.Outcome(); got != c.want {
			t.Errorf("%+v: got %q want %q", c.s, got, c.want)
		}
	}
}
