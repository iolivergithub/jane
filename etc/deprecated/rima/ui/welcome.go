package ui

import (
	"fmt"
	"runtime"

	"rima/configuration"
	"rima/database"
)

// Provides the standard welcome message to stdout.
func WelcomeMessage(VERSION string, BUILD string) {
	lensdb := database.DBSize()

	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  RIMA\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("|   + Listening on port %v bound to %v\n", configuration.ConfigData.Port, configuration.ConfigData.ListenOn)
	fmt.Printf("|   + Jane is at %v\n", configuration.ConfigData.JaneURL)
	fmt.Printf("|   + DB %d entries in %v \n", lensdb, configuration.ConfigData.DBFile)
	fmt.Printf("|   + Scripts at %v\n", configuration.ConfigData.ScriptDir)
	fmt.Printf("+========================================================\n")
}
