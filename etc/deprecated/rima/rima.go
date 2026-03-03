package main

import (
	"flag"
	"fmt"
	"net/http"

	"rima/configuration"
	"rima/database"
	"rima/restapi"
	"rima/ui"
)

// Version number
const VERSION string = "v0.1 RIMA"

// the BUILD value can be set during compilation.
var BUILD string = "not set"

// Command line flags
var janeURL string
var port string
var dbfile string
var scriptDir string
var listenOn string

func main() {
	flag.StringVar(&janeURL, "jane", "http://127.0.0.1:8520", "Address of Jane's REST API, eg: http://127.0.0.1:8540")
	flag.StringVar(&port, "port", "8522", "Port on which Rima runs, defaults to 8522")
	flag.StringVar(&dbfile, "db", "./rima.db", "Location of DB file, defaults to ./rima.db")
	flag.StringVar(&scriptDir, "scripts", "./rimascripts/", "Location (path to director) of scripts, defaults to ./rimascripts/")
	flag.StringVar(&listenOn, "listen", "0.0.0.0", "Which address should Rima listen on, defaults to 0.0.0.0")

	flag.Parse()
	configuration.ConfigData = &configuration.ConfigurationStruct{JaneURL: janeURL, Port: port, DBFile: dbfile, ScriptDir: scriptDir, ListenOn: listenOn}

	database.SetupSDB()
	ui.WelcomeMessage(VERSION, BUILD)

	http.HandleFunc("/", restapi.RootHandler)
	http.HandleFunc("/pcall", restapi.PcallHandler)
	http.HandleFunc("/status", restapi.StatusHandler)

	http.ListenAndServe(fmt.Sprintf("%v:%v", listenOn, port), nil)
}
