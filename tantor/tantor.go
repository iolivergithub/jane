package main

import (
	"flag"
	"fmt"

	"tantor/actions"
	"tantor/provisioningfile"
)

const VERSION string = "v0.1 TANTOR"

var BUILD string = "not set"

func main() {
	fmt.Println("Starting")
	flag.Parse()

	if len(flag.Args()) != 1 {
		panic("No provisioning file specified")
	}

	fmt.Printf("Reading provisioning data from file %v\n", flag.Arg(0))
	provisioningfile.ReadProvisioningFile(flag.Arg(0))

	fmt.Println("Collecting worklist")
	worklist := provisioningfile.ProvisioningData.ProvisioningWorkList

	for k, v := range worklist {
		fmt.Printf(" ... %v : %v\n", k, v)
	}

	actions.RunWorklist(worklist)

	fmt.Println("%v\n", provisioningfile.ProvisioningData)

}
