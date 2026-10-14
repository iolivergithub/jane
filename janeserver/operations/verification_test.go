package operations

import (
	"errors"
	"testing"

	"a10/structures"
)

// The expected value fetched for a rule must be returned so that Verify can
// store it in the result (issues #33, #51: it used to be lost to a shadowed
// variable, leaving every stored result without its expected value).
func TestEvaluateRuleReturnsExpectedValue(t *testing.T) {
	want := structures.ExpectedValue{ItemID: "ev-1", Name: "golden PCRs", IntentID: "intent-1"}
	var seenByRule structures.ExpectedValue

	rule := structures.Rule{
		Name:    "test_rule",
		NeedsEV: true,
		CallFunction: func(c structures.Claim, name string, ev structures.ExpectedValue, s structures.Session, ps map[string]interface{}) (structures.ResultValue, string, error) {
			seenByRule = ev
			return structures.Success, "ok", nil
		},
	}
	getev := func(structures.Claim, structures.Rule) (structures.ExpectedValue, error) { return want, nil }

	ev, rv, msg := evaluateRule(structures.Claim{ItemID: "claim-1"}, rule, structures.Session{}, nil, getev)

	if ev.ItemID != want.ItemID || ev.IntentID != want.IntentID {
		t.Errorf("returned expected value = %+v, want %+v", ev, want)
	}
	if seenByRule.ItemID != want.ItemID {
		t.Errorf("rule saw expected value %+v, want %+v", seenByRule, want)
	}
	if rv != structures.Success || msg != "ok" {
		t.Errorf("result = %v %q, want Success \"ok\"", rv, msg)
	}
}

func TestEvaluateRuleMissingExpectedValue(t *testing.T) {
	called := false
	rule := structures.Rule{
		Name:    "test_rule",
		NeedsEV: true,
		CallFunction: func(structures.Claim, string, structures.ExpectedValue, structures.Session, map[string]interface{}) (structures.ResultValue, string, error) {
			called = true
			return structures.Success, "", nil
		},
	}
	getev := func(structures.Claim, structures.Rule) (structures.ExpectedValue, error) {
		return structures.ExpectedValue{}, errors.New("not found")
	}

	ev, rv, _ := evaluateRule(structures.Claim{ItemID: "claim-1"}, rule, structures.Session{}, nil, getev)

	if rv != structures.MissingExpectedValue {
		t.Errorf("result = %v, want MissingExpectedValue", rv)
	}
	if ev.ItemID != "" {
		t.Errorf("expected value = %+v, want empty", ev)
	}
	if called {
		t.Error("rule was called despite the missing expected value")
	}
}

func TestEvaluateRuleClaimError(t *testing.T) {
	rule := structures.Rule{Name: "test_rule"} // no call function: must not be reached
	getev := func(structures.Claim, structures.Rule) (structures.ExpectedValue, error) {
		t.Error("expected value fetched for a claim error")
		return structures.ExpectedValue{}, nil
	}

	_, rv, _ := evaluateRule(structures.Claim{ItemID: "claim-1", BodyType: structures.CLAIMERROR}, rule, structures.Session{}, nil, getev)

	if rv != structures.VerifyClaimErrorAttempt {
		t.Errorf("result = %v, want VerifyClaimErrorAttempt", rv)
	}
}
