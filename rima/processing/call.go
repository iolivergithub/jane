package processing

import (
	"fmt"
	"os/exec"

	"rima/configuration"
	"rima/database"
)

func CallScript(url string, eid string, epn string, pol string, msg string) (string, string, error) {
	fmt.Printf("Call string %v, %v, %v, %v\n", eid, epn, pol, msg)

	dbe, ok := database.GetEntry(eid, epn, pol)
	fmt.Printf(" ---> entry %v, %v\n", dbe, ok)

	scriptlocation := fmt.Sprintf("%v/%v", configuration.ConfigData.ScriptDir, dbe)
	fmt.Printf(" ---> script %v\n\n", scriptlocation)

	cmd := exec.Command(scriptlocation, url, eid, epn, pol, msg)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("\n\nerror is %v\n", err.Error())
	}

	//instead of alice we should be taking the contents of the last line - which according to convention should just be a sessionid
	return "alice", string(out), nil
}
