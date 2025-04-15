package main

import (
	"flag"
	"fmt"
	"runtime"

	"tantor/actions"
	"tantor/janeapi"
	"tantor/provisioningfile"
)

const VERSION string = "v0.1 TANTOR"

var BUILD string = "not set"

func main() {
	welcomeMessage()
	flag.Parse()

	if len(flag.Args()) != 1 {
		panic("No provisioning file specified")
	}

	fmt.Printf("Reading provisioning data from file %v\n", flag.Arg(0))
	provisioningfile.ReadProvisioningFile(flag.Arg(0))

	s := janeapi.GetServerStatus()
	fmt.Printf("Server status %s\n", s)

	fmt.Println("Collecting worklist")
	worklist := provisioningfile.ProvisioningData.ProvisioningWorkList

	actions.RunWorklist(worklist)

	exitMessage()

}

// Provides the standard welcome message to stdout.
func welcomeMessage() {
	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  Tantor\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("+========================================================\n")
}

func exitMessage() {
	fmt.Printf("\n")
	fmt.Printf("+========================================================================================\n")
	fmt.Printf("|  Tanor\n")
	fmt.Printf("|  Hwyl fawr!\n")
	fmt.Printf("+========================================================================================\n\n")
}
