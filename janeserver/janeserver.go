// Attestation Engine A10 JANE, February 2024 onwards.
// The main package starts the various interfaces: REST, MQTT and links to the database system
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	_ "sync"
	"syscall"

	"a10/configuration"
	"a10/datalayer"
	"a10/logging"
	"a10/protocols"
	"a10/rules"
	"a10/utilities"

	"a10/services/restapi"
	"a10/services/webui"
	"a10/services/x3270"
)

// Version number
const VERSION string = "v1.01 JANE"

// the BUILD value can be set during compilation.
var BUILD string = "not set"

// and we generate a unique identifier for this whole run session
var RUNSESSION string = utilities.MakeID()

// Command line flags
var flagREST = flag.Bool("startREST", true, "Start the REST API, defaults to true")
var flagWEB = flag.Bool("startWebUI", true, "Start the HTML Web UI, defaults to true")
var flagX3270 = flag.Bool("startx3270", true, "Start the X3270 UI, defaults to true")

var configFile = flag.String("config", "./config.yaml", "Location and name of the configuration file, default to a config.yaml in the current directory")

// Provides the standard welcome message to stdout.
func welcomeMessage() {
	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  JANESERVER version\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("|   + runing with name %v\n", configuration.ConfigData.System.Name)
	fmt.Printf("|   + session identifier is %v\n", RUNSESSION)
	fmt.Printf("+========================================================\n")
}

// This starts everything...here we "go" <- great pun! :-)
func main() {
	// we need to see what is on the command line and process the configuration file
	// If this fails, we panic
	flag.Parse()
	configuration.SetupConfiguration(*configFile)

	// now we know where things are, we can initialise the datalayer, ie: database, messaging etc
	// if this fails, we panic
	datalayer.InitialiseDatalayer()

	// Ok, we're up...let's log this.
	msg := fmt.Sprintf("Starting: %v, build %v, OS %v, ARCH %v", VERSION, BUILD, runtime.GOOS, runtime.GOARCH)
	logging.MakeLogEntry("SYS", "startup/INIT", RUNSESSION, configuration.ConfigData.System.Name, msg)
	msg = fmt.Sprintf("Command line contained %v items: %v", len(os.Args), os.Args)
	logging.MakeLogEntry("SYS", "startup/INIT", RUNSESSION, "command line", msg)

	welcomeMessage()

	// initialise the internal parts of the system, ie: rules and protocols.
	// If the datalayer have come up properly, but some other external error has occured, eg: authorisation etc,
	// then we will get a panic from these below.

	protocols.RegisterProtocols()
	rules.RegisterRules()

	// and if this has gone well...

	msg = fmt.Sprintf("DB,MQTT,Rules initialised. Starting services: web %v, rest %v, x3720 %v", *flagWEB, *flagREST, *flagX3270)
	logging.MakeLogEntry("SYS", "startup", RUNSESSION, configuration.ConfigData.System.Name, msg)

	// start the internal services
	internalservices()

	logging.MakeLogEntry("SYS", "shutdown", configuration.ConfigData.System.Name, "JANE "+VERSION, "Final message: We apologise for the inconvience (bring 42 towels)")
	fmt.Println("+=== Final message: We apologise for the inconvience (42). Next time, bring a towel ===")

}

func internalservices() {
	// Start (or not) the various internal services
	// As these run as threads, we put them in a wait group
	// Need to implement a proper graceful shutdown mechanism
	//
	// If any of these internal services fail to start, then the system may panic

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called at the end to clean up

	// Channel to listen for system signals (e.g., Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Bring up the services
	if *flagX3270 == true {
		go x3270.StartX3270(ctx)
	}
	if *flagREST == true {
		go restapi.StartRESTInterface(ctx)
	}
	if *flagWEB == true {
		go webui.StartWebUI(ctx)
	}

	// Wait for an interrupt signal to initiate graceful shutdown
	select {
	case <-sigChan:
		// Handle shutdown signal (Ctrl+C or SIGTERM)
		logging.MakeLogEntry("SYS", "shutdown", configuration.ConfigData.System.Name, "JANE "+VERSION, "Received shutdown signal. Shutting down gracefully")
		fmt.Println("+=== Received shutdown signal. Shutting down gracefully ===")

		// Cancel the context to notify all goroutines to stop
		cancel()
	}

}
