package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"a10/rules/chares"
	"a10/rules/sysrules"
	"a10/rules/tpm2rules"
	"a10/rules/uefirules"
)

// Every intent and rule named in jp's skeletons (#125), including the
// commented-out ones, is a standard intent and a registered rule.
func TestSkeletonNamesExist(t *testing.T) {
	var intents []struct {
		ItemID string `json:"itemid"`
	}
	b, err := os.ReadFile("../../etc/standardintents/standardintents.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &intents); err != nil {
		t.Fatal(err)
	}
	knownIntents := map[string]bool{}
	for _, i := range intents {
		knownIntents[i.ItemID] = true
	}
	knownRules := map[string]bool{}
	for _, r := range append(append(append(tpm2rules.Registration(), uefirules.Registration()...), sysrules.Registration()...), chares.Registration()...) {
		knownRules[r.Name] = true
	}

	intentRE := regexp.MustCompile(`std::intent::[A-Za-z0-9:]+[A-Za-z0-9]`)
	ruleRE := regexp.MustCompile(`\b(tpm2|sys|UEFI|chares)_[A-Za-z0-9]+\b`)

	files, err := filepath.Glob("../../provisioner/skeleton/*.yaml")
	if err != nil || len(files) != 4 {
		t.Fatalf("skeletons: %v %v", files, err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range intentRE.FindAllString(string(b), -1) {
			if !knownIntents[i] {
				t.Errorf("%s: %s is not a standard intent", filepath.Base(f), i)
			}
		}
		for _, r := range ruleRE.FindAllString(string(b), -1) {
			if !knownRules[r] {
				t.Errorf("%s: %s is not a rule", filepath.Base(f), r)
			}
		}
	}
}
