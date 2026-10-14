package webui

import (
	"testing"

	"a10/structures"
)

// Results stored without an expected value (all results verified before the
// fix for #33/#51) still show their intent and element, taken from the claim.
func TestResultPageIntentFromClaim(t *testing.T) {
	lk := &lookups{
		Intent: func(id string) (structures.Intent, bool) {
			if id == "intent-1" {
				return structures.Intent{ItemID: id, Name: "tpm2/quote"}, true
			}
			return structures.Intent{}, false
		},
	}
	r := structures.Result{ItemID: "r1", ClaimID: "c1", RuleName: "tpm2_validQuote"} // no expected value
	claim := structures.Claim{ItemID: "c1", Header: structures.ClaimHeader{
		Intent:  structures.Intent{ItemID: "intent-1", Name: "old name"},
		Element: structures.Element{ItemID: "e1", Name: "pi4"},
	}}

	page := buildResultPage(r, nil, claim, lk)

	if page.Intent != (ref{"intent-1", "tpm2/quote"}) {
		t.Errorf("intent = %+v, want the current intent record", page.Intent)
	}
	if page.Element != (ref{"e1", "pi4"}) {
		t.Errorf("element = %+v, want the claim's element", page.Element)
	}
}

// When the result carries its expected value, the intent comes from it and
// the expected value is shown.
func TestResultPageIntentFromExpectedValue(t *testing.T) {
	lk := &lookups{}
	ev := structures.ExpectedValue{ItemID: "ev1", Name: "golden", IntentID: "intent-2", ElementID: "e2"}
	r := structures.Result{ItemID: "r1", ClaimID: "c1", ExpectedValue: ev}
	claim := structures.Claim{ItemID: "c1", Header: structures.ClaimHeader{Intent: structures.Intent{ItemID: "intent-1"}}}

	page := buildResultPage(r, nil, claim, lk)

	if page.Intent.ID != "intent-2" {
		t.Errorf("intent = %+v, want intent-2 from the expected value", page.Intent)
	}
	if page.EV.ItemID != "ev1" {
		t.Errorf("expected value = %+v, want ev1", page.EV)
	}
}
