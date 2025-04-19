package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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
	flag.StringVar(&scriptDir, "scripts", "./rscripts/", "Location (path to director) of scripts, defaults to ./rscripts/")
	flag.StringVar(&listenOn, "listen", "0.0.0.0", "Which address should Rima listen on, defaults to 0.0.0.0")

	flag.Parse()

	setupSDB(dbfile)
	welcomeMessage()

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/pcall", pcallHandler)

	http.ListenAndServe(fmt.Sprintf("%v:%v", listenOn, port), nil)
}

func welcomeMessage() {
	fmt.Printf("\n")
	fmt.Printf("+========================================================\n")
	fmt.Printf("|  RIMA\n")
	fmt.Printf("|   + %v O/S on %v\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("|   + version %v, build %v\n", VERSION, BUILD)
	fmt.Printf("|   + Listening on port %v bound to %v\n", port, listenOn)
	fmt.Printf("|   + Jane is at %v\n", janeURL)
	fmt.Printf("|   + DB %d entries in %v \n", len(SDB), dbfile)
	fmt.Printf("|   + Scripts at %v\n", scriptDir)
	fmt.Printf("+========================================================\n")
}

//**************************************************************************
//
// REST API Handlers
//
//**************************************************************************

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome")
}

type pcallRequest struct {
	EID string `json:"eid"`
	Pol string `json:"pol"`
	Msg string `json:"msg"`
}

type pcallResponse struct {
	Sid string `json:"sid"`
	Out string `json:"out"`
}

type pcallErrorResponse struct {
	Error string `json:"error"`
	Out   string `json:"out"`
}

func pcallHandler(w http.ResponseWriter, r *http.Request) {
	var data pcallRequest

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	sid, out, err := callScript(janeURL, data.EID, data.Pol, data.Msg)

	fmt.Printf("sid, out, err: %v,%v,%v\n", sid, out, err)

	if err != nil {
		eresponse := pcallErrorResponse{Error: err.Error(), Out: out}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(eresponse); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	response := pcallResponse{Sid: sid, Out: out}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//**************************************************************************
//
// Database
//
//**************************************************************************

type sdbKey struct {
	Eid string
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
	for _, line := range data {
		SDB[sdbKey{line[0], line[1]}] = line[2]
	}
}

func getEntry(eid string, pol string) (string, bool) {
	val, ok := SDB[sdbKey{eid, pol}]
	return val, ok
}

//**************************************************************************
//
// Script Call Handlers
//
//**************************************************************************

func callScript(url string, eid string, pol string, msg string) (string, string, error) {
	fmt.Printf("Call string %v, %v, %v", eid, pol, msg)

	dbe, ok := getEntry(eid, pol)
	fmt.Printf(", entry %v, %v\n\n", dbe, ok)

	cmd := exec.Command(dbe, url, eid, pol, msg)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("\n\nerror is %v\n", err.Error())
	}
	return "alice", string(out), nil
}
