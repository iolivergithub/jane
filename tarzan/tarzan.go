// The main package starts the various interfaces: REST, MQTT and links to the database system
package main

import (
	"flag"
	"fmt"
	"runtime"

	"ta10/common"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"ta10/ima"
	"ta10/sys"
	"ta10/tpm2"
	"ta10/uefi"
)

// Version number
const VERSION string = "v0.2"

var BUILD string = "not set"

const PREFIX = ""

// Provides the standard welcome message to stdout.
func welcomeMessage(unsafe bool) {
	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  Tarzan\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("|   + session identifier is %v\n", utilities.RUNSESSION)
	fmt.Printf("|   + unsafe mode? %v\n", unsafe)
	fmt.Printf("+========================================================\n")
}

func exitMessage() {
	fmt.Printf("\n")
	fmt.Printf("+========================================================================================\n")
	fmt.Printf("|  Tarzan\n")
	fmt.Printf("|   + session identifier was %v\n", utilities.RUNSESSION)
	fmt.Printf("|  Hwyl fawr!\n")
	fmt.Printf("+========================================================================================\n\n")
}

func checkUnsafeMode(unsafe bool) {
	if unsafe == true {
		utilities.SetUnsafeMode()

		fmt.Printf("\n")
		fmt.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n")
		fmt.Printf("TA10 is running in UNSAFE file access mode.  Unsafe is set to %v\n", utilities.IsUnsafe())
		fmt.Printf("Requests for log files, eg: UEFI, IMA, that supply a non default location will happily read that file\n")
		fmt.Printf("This is a HUGE security issue. YOU HAVE BEEN WARNED\n")
		fmt.Printf("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n")

	}
}

// These configure the rest API

func startRESTInterface(sys, tpm2, uefi, ima, txt bool, p *string) {
	router := echo.New()
	router.HideBanner = true

	//not necessary, but I will keep this here because this is now my example of how to use middlewares
	//in echo, plus the import declaration above
	//
	// Of the two below, the gzip is the only useful one. The BodyDump was used for debugging
	//
	//router.Use(middleware.BodyDump(func(c echo.Context,reqBody,resBody []byte) {} ))
	router.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: 5}))

	if sys == true {
		fmt.Println("   +-- Sys attestation API enabled")
		setupSYSendpoints(router)
	}
	if uefi == true {
		fmt.Println("   +-- UEFI attestation API enabled")
		setupUEFIendpoints(router)
	}
	if ima == true {
		fmt.Println("   +-- IMA attestation API enabled")
		setupIMAendpoints(router)
	}

	if tpm2 == true {
		fmt.Println("   +-- TPM2 attestation API enabled")
		setupTPM2endpoints(router)
	}

	/*    if txt == true {
	      	fmt.println("   +-- TXT attestation API enabled")
	      	setupTXTendpoints(router)
	      }
	*/

	if sys == false && uefi == false && ima == false && tpm2 == false && txt == false {
		fmt.Println("   +-- WARNING: tarzan isn't listening to anyone and won't respond")
	}

	//get configuration data
	port := ":" + *p
	//crt := configuration.ConfigData.Rest.Crt
	//key:= configuration.ConfigData.Rest.Key
	usehttp := true

	//start the server
	if usehttp == true {
		fmt.Printf("   +-- HTTP interface on port %v enabled\n", port)
		router.Logger.Fatal(router.Start(string(port)))

	} else {
		fmt.Printf("   +-- HTTPS interface on port %v enabled\n", port)
		//router.Logger.Fatal(router.StartTLS(port,crt,key))
	}
}

func setupSYSendpoints(router *echo.Echo) {
	router.POST(PREFIX+"/sys/info", sys.Sysinfo)
}

func setupUEFIendpoints(router *echo.Echo) {
	router.POST(PREFIX+"/uefi/eventlog", uefi.Eventlog)
	router.POST(PREFIX+"/uefi/efivars", uefi.Efivars)
	router.POST(PREFIX+"/uefi/bootconfig", uefi.BootConfig)
}

func setupIMAendpoints(router *echo.Echo) {
	router.POST(PREFIX+"/ima/asciilog", ima.ASCIILog)
}

func setupTPM2endpoints(router *echo.Echo) {
	router.POST(PREFIX+"/tpm2/newpcrs", tpm2.NewPCRs)
	router.POST(PREFIX+"/tpm2/pcrs", tpm2.PCRs)
	router.POST(PREFIX+"/tpm2/quote", tpm2.Quote)
}

// This starts everything...here we "go" :-)
func main() {
	utilities.RUNSESSION = utilities.MakeID()

	flagSYS := flag.Bool("sys", false, "Expose the sys attestation API")
	flagTPM2 := flag.Bool("tpm2", false, "Expose the tpm2 attesation API")
	flagUEFI := flag.Bool("uefi", false, "Expose the uefi attestation API")
	flagIMA := flag.Bool("ima", false, "Expose the ima attestation API")
	flagTXT := flag.Bool("txt", false, "Expose the txt attestation API")

	flagUNSAFEFILEACCESS := flag.Bool("unsafe", false, "Allow caller to request ANY file instead of the default UEFI and IMA locations. THIS IS UNSAFE!")

	flagPort := flag.String("port", "8530", "Run the TA on the given port. Defaults to 8530")

	flag.Parse()

	welcomeMessage(*flagUNSAFEFILEACCESS)
	checkUnsafeMode(*flagUNSAFEFILEACCESS)

	startRESTInterface(*flagSYS, *flagTPM2, *flagUEFI, *flagIMA, *flagTXT, flagPort)
	exitMessage()
}
