package rules

import (
	"a10/operations"
	"a10/structures"

	"a10/rules/chares"
	"a10/rules/nullrule"
	"a10/rules/sysrules"
	"a10/rules/tpm2rules"
	"a10/rules/uefirules"

	"a10/rules/testcontainerrules"
)

func RegisterRules() {
	registerListOfRules(nullrule.Registration())
	registerListOfRules(tpm2rules.Registration())
	registerListOfRules(uefirules.Registration())
	registerListOfRules(testcontainerrules.Registration())
	registerListOfRules(sysrules.Registration())
	registerListOfRules(chares.Registration())

}

func registerListOfRules(rs []structures.Rule) {
	for _, r := range rs {
		operations.AddRule(r)
	}
}
