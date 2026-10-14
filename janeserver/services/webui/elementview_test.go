package webui

import (
	"strings"
	"testing"

	"a10/structures"
)

// The same rule applied to claims of different intents in one session must
// give one history row each, not a single merged row (#53).
func TestElementMatrixRuleRowsPerIntent(t *testing.T) {
	sess := structures.Session{ItemID: "s1"}
	claim := func(id, intent string) structures.Claim {
		return structures.Claim{ItemID: id, BodyType: "tpm2", Header: structures.ClaimHeader{
			Session: sess,
			Intent:  structures.Intent{ItemID: intent, Name: intent},
			Timing:  structures.Timing{Requested: 100},
		}}
	}
	result := func(id, claimID string, rv structures.ResultValue) structures.Result {
		return structures.Result{ItemID: id, ClaimID: claimID, Session: sess, RuleName: "sys_machineIDMatches", VerifiedAt: 200, Result: rv}
	}

	cs := []structures.Claim{claim("c1", "sys/info"), claim("c2", "sys/machineid")}
	rs := []structures.Result{result("r1", "c1", structures.Success), result("r2", "c2", structures.Fail)}

	p := buildElementPage(structures.Element{ItemID: "e1"}, cs, rs, nil)

	if len(p.RuleRows) != 2 {
		labels := []string{}
		for _, r := range p.RuleRows {
			labels = append(labels, r.Label)
		}
		t.Fatalf("got %d rule rows %q, want 2 (one per intent)", len(p.RuleRows), labels)
	}
	want := map[string]string{
		"sys_machineIDMatches · sys/info":      "/result/r1",
		"sys_machineIDMatches · sys/machineid": "/result/r2",
	}
	for _, row := range p.RuleRows {
		link, ok := want[row.Label]
		if !ok {
			t.Errorf("unexpected row %q", row.Label)
			continue
		}
		if len(row.Cells) != 1 || row.Cells[0].Link != link {
			t.Errorf("row %q cells = %+v, want one cell linking %s", row.Label, row.Cells, link)
		}
		if !strings.Contains(row.Cells[0].Title, "intent ") {
			t.Errorf("row %q hover text %q does not name the intent", row.Label, row.Cells[0].Title)
		}
	}
}

// Rule rows are grouped by rule type, case-insensitively, and each group's
// summary cell holds the worst outcome in that session (#59).
func TestGroupRuleRows(t *testing.T) {
	timeline := []elementSession{{ID: "s1", Label: "a"}, {ID: "s2", Label: "b"}, {ID: "s3", Label: "c"}}
	row := func(label string, outcomes ...string) matrixRow {
		r := matrixRow{Label: label}
		for _, o := range outcomes {
			r.Cells = append(r.Cells, matrixCell{Outcome: o})
		}
		return r
	}
	rows := []matrixRow{
		row("UEFI_PK", outcomePass, outcomePass, ""),
		row("sys_machineID_Agent · std::intent::sys::info", outcomePass, outcomeOther, ""),
		row("tpm2_firmware", outcomePass, outcomeOther, ""),
		row("tpm2_safe", outcomeFail, outcomePass, ""),
		row("uefi_lowercase", "", outcomeFail, ""),
		row("norule", outcomePass, "", ""),
	}

	gs := groupRuleRows(rows, timeline)

	names := []string{}
	for _, g := range gs {
		names = append(names, g.Name)
	}
	if got, want := strings.Join(names, ","), "other,sys,tpm2,UEFI"; got != want {
		t.Fatalf("groups = %s, want %s", got, want)
	}

	tpm2 := gs[2]
	if len(tpm2.Rows) != 2 {
		t.Fatalf("tpm2 has %d rows, want 2", len(tpm2.Rows))
	}
	if got := []string{tpm2.Summary[0].Outcome, tpm2.Summary[1].Outcome, tpm2.Summary[2].Outcome}; got[0] != outcomeFail || got[1] != outcomeOther || got[2] != "" {
		t.Errorf("tpm2 summary = %q, want [fail other \"\"]", got)
	}
	if tpm2.Summary[0].Link != "/session/s1" {
		t.Errorf("summary link = %q, want /session/s1", tpm2.Summary[0].Link)
	}
	if !strings.Contains(tpm2.Summary[0].Title, "1 pass, 1 fail, 0 other") {
		t.Errorf("summary title = %q", tpm2.Summary[0].Title)
	}

	uefi := gs[3]
	if len(uefi.Rows) != 2 || uefi.Summary[1].Outcome != outcomeFail {
		t.Errorf("UEFI group = %d rows, session 2 %q; want 2 rows (UEFI_ and uefi_ together), fail", len(uefi.Rows), uefi.Summary[1].Outcome)
	}
	if gs[0].ID == gs[1].ID {
		t.Errorf("group ids are not unique: %q", gs[0].ID)
	}
}
