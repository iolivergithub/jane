package ui

import (
	"fmt"
	"runtime"
)

// Provides the standard welcome message to stdout.
func WelcomeMessage(VERSION string, BUILD string, port string, listenOn string, janeURL string, lensdb int, dbfile string, scriptDir string) {
	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  RIMA\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("|   + Listening on port %v bound to %v\n", port, listenOn)
	fmt.Printf("|   + Jane is at %v\n", janeURL)
	fmt.Printf("|   + DB %d entries in %v \n", lensdb, dbfile)
	fmt.Printf("|   + Scripts at %v\n", scriptDir)
	fmt.Printf("+========================================================\n")
}
