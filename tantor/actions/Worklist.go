package actions

import (
	"fmt"
	"tantor/provisioningfile"
)

func RunWorklist(ws []string) {
	for j, v := range ws {
		fmt.Printf("Running item %v : %v\n", j, v)

		switch {
		case v == "collectsysinfo":
			h, _ := CollectSystemInfo()
			provisioningfile.ProvisioningData.Element.Host = h
			successmessage()
		case v == "collectuefi":
			f, err := CollectUEFIEventLogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.UEFI.Eventlog = f
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "collectuefi":
			f, err := CollectIMALogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.IMA.ASCIILog = f
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "collectima":
			f, err := CollectIMALogLocation()
			if err == nil {
				provisioningfile.ProvisioningData.Element.IMA.ASCIILog = f
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "tpmclear":
			_, err := TPMClear()
			if err == nil {
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "tpmprovision":
			_, err := TPMProvision()
			if err == nil {
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "createevs":
			_, err := CreateEVS()
			if err == nil {
				successmessage()
			} else {
				errormessage(err)
			}
		case v == "createelement":
			_, err := CreateElement()
			if err == nil {
				successmessage()
			} else {
				errormessage(err)
			}
		default:
			fmt.Println("Error: unknown work request")
		}
	}
}

func successmessage() {
	fmt.Println("* success")
}

func errormessage(e error) {
	fmt.Printf("X error %w\n", e.Error())
}
