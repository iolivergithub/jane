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
