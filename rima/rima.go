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
	flag.StringVar(&scriptDir, "scripts", "./rimascripts/", "Location (path to director) of scripts, defaults to ./rimascripts/")
	flag.StringVar(&listenOn, "listen", "0.0.0.0", "Which address should Rima listen on, defaults to 0.0.0.0")

	flag.Parse()

	setupSDB(dbfile)
	welcomeMessage()

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/pcall", pcallHandler)
	http.HandleFunc("/status", statusHandler)

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
	fmt.Fprintln(w, "Welcome - Rima is running")
}

type pcallRequest struct {
	EID string `json:"eid"`
	EPN string `json:"epn"`
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

	sid, out, err := callScript(janeURL, data.EID, data.EPN, data.Pol, data.Msg)

	fmt.Printf("sid: %v\nout: %v\n err: %v\n", sid, out, err)

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

type rimaStatusResponse struct {
	JaneURL   string `json:"janeurl"`
	Port      string `json:"port"`
	DBFile    string `json:"dbfile"`
	ScriptDir string `json:"scriptDir"`
	ListedOn  string `json:"listenOn"`
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	response := rimaStatusResponse{JaneURL: janeURL, Port: port, DBFile: dbfile, ScriptDir: scriptDir, ListedOn: listenOn}

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
