package sysrules

import (
	"a10/structures"
	"fmt"
	"strings"
)

func Registration() []structures.Rule {

	ruleS := structures.Rule{"sys_taRunningSafely", "Checks that the TA is NOT in unsafe mode of operation", Callrulesafe, false}
	ruleMTA := structures.Rule{"sys_machineID_Agent", "Checks that the Machine ID in /etc/machineid has not been changed - obtained from trust agent", CallrulemachineIDTA, true}
	ruleMCR := structures.Rule{"sys_machineID_CrossRef", "Checks that the Machine ID in /etc/machineid has not been changed - cross-referenced from Element description", CallrulemachineIDCR, false}

	return []structures.Rule{ruleS, ruleMTA, ruleMCR}
}

func Callrulesafe(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {

	unsafevalue, ok := claim.Body["unsafe"]
	if !ok {
		return structures.RuleCallFailure, "TA not of correct type, or not reporting unsafe parameter value", nil
	}

	if unsafevalue == true {
		return structures.Fail, "TA operating in UNSAFE mode", nil
	} else {
		return structures.Success, "TA operating in safe mode", nil
	}
}

func CallrulemachineIDTA(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {

	machineid, ok := claim.Body["machineid"]
	if !ok {
		return structures.RuleCallFailure, "TA not of correct type, or not reporting machineid parameter value", nil
	}

	claimedMachineID := strings.Trim(fmt.Sprintf("%v", machineid), " \t\n")

	// We get this from the EXPECTED VALUES
	expectedMachineID := (ev.EVS)["machineid"]

	fmt.Printf("Comparison\n%v\n%v\n%v\n===\n", claimedMachineID, expectedMachineID, claimedMachineID == expectedMachineID)

	// and now the check
	if expectedMachineID == claimedMachineID {
		return structures.Success, "MachineID  matches", nil
	} else {
		msg := fmt.Sprintf("Got %v as Machine ID version but expected %v", claimedMachineID, expectedMachineID)
		return structures.Fail, msg, nil
	}

}

func CallrulemachineIDCR(claim structures.Claim, rule string, ev structures.ExpectedValue, session structures.Session, parameter map[string]interface{}) (structures.ResultValue, string, error) {

	machineid, ok := claim.Body["machineid"]
	if !ok {
		return structures.RuleCallFailure, "TA not of correct type, or not reporting machineid parameter value", nil
	}

	claimedMachineID := strings.Trim(fmt.Sprintf("%v", machineid), " \t\n")

	// We get this from the CLAIM.ELEMENT.HOST.MACHINEID
	expectedMachineID := claim.Header.Element.Host.MachineID
	fmt.Printf("Claim machine ID is %v\n", expectedMachineID)

	fmt.Printf("Comparison\n%v\n%v\n%v\n===\n", claimedMachineID, expectedMachineID, claimedMachineID == expectedMachineID)

	// and now the check
	if expectedMachineID == claimedMachineID {
		return structures.Success, "MachineID  matches", nil
	} else {
		msg := fmt.Sprintf("Got %v as Machine ID version but expected %v", claimedMachineID, expectedMachineID)
		return structures.Fail, msg, nil
	}

}
