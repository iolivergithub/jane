package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"

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

	setupSDB(dbfile)
	ui.WelcomeMessage(VERSION, BUILD, port, listenOn, janeURL, len(SDB), dbfile, scriptDir)

	http.HandleFunc("/", restapi.RootHandler)
	http.HandleFunc("/pcall", restapi.PcallHandler)
	http.HandleFunc("/status", restapi.StatusHandler)

	http.ListenAndServe(fmt.Sprintf("%v:%v", listenOn, port), nil)
}

//**************************************************************************
//
// Database
//
//**************************************************************************

type sdbKey struct {
	Eid string
	Epn string
	Pol string
}

var SDB map[sdbKey]string

func setupSDB(fn string) {
	SDB = make(map[sdbKey]string)

	// Load in DB
	fmt.Printf("loading %v\n", fn)

	f, err := os.Open(dbfile)
	if err != nil {
		panic(fmt.Sprintf("DB file %v does not exist.\n", dbfile))
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	dbdata, err := csvReader.ReadAll()
	if err != nil {
		panic(fmt.Sprintf("DB file is corrupt. Error is %v\n", err.Error()))
	}

	populateSDB(dbdata)

	fmt.Printf("SBD\n%v\n", SDB)
}

// Format of DB is   ElementItemID, Policy identifier, script name
func populateSDB(data [][]string) {
	for j, line := range data {
		fmt.Printf(" Entry #%v is %v %v %v\n", j, line[0], line[1], line[2], line[3])
		SDB[sdbKey{line[0], line[1], line[2]}] = line[3]
	}
}

func getEntry(eid string, epn string, pol string) (string, bool) {
	val, ok := SDB[sdbKey{eid, epn, pol}]
	return val, ok
}

//**************************************************************************
//
// Script Call Handlers
//
//**************************************************************************

func callScript(url string, eid string, epn string, pol string, msg string) (string, string, error) {
	fmt.Printf("Call string %v, %v, %v, %v\n", eid, epn, pol, msg)

	dbe, ok := getEntry(eid, epn, pol)
	fmt.Printf(" ---> entry %v, %v\n", dbe, ok)

	scriptlocation := fmt.Sprintf("%v/%v", scriptDir, dbe)
	fmt.Printf(" ---> script %v\n\n", scriptlocation)

	cmd := exec.Command(scriptlocation, url, eid, epn, pol, msg)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("\n\nerror is %v\n", err.Error())
	}

	//instead of alice we should be taking the contents of the last line - which according to convention should just be a sessionid
	return "alice", string(out), nil
}
