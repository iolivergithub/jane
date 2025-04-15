package main

import(
	"fmt"
	"net/http"
	"encoding/json"
	"os/exec"
)

func main(){
	setupSDB()

	http.HandleFunc("/",rootHandler)
	http.HandleFunc("/pcall", pcallHandler)

	port := 8541
	fmt.Println("Running")
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d",port), nil)
}


//**************************************************************************
//
// REST API Handlers
//
//**************************************************************************

func rootHandler(w http.ResponseWriter, r *http.Request){
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
    Out string `json:"out"`	 
}

func pcallHandler(w http.ResponseWriter, r *http.Request){
	var data pcallRequest

	err := json.NewDecoder(r.Body).Decode(&data)
	if err!=nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	sid, out, err := callScript(data.EID, data.Pol, data.Msg)
	if err != nil {
 		eresponse := pcallErrorResponse{ Error:err.Error(), Out:out }
		w.Header().Set("Content-Type","application/json")
 		if err := json.NewEncoder(w).Encode(eresponse); err != nil {
 			http.Error(w, err.Error(), http.StatusInternalServerError)
 			return
 		}   
	}

 	response := pcallResponse{ Sid:sid, Out:out }
 	w.Header().Set("Content-Type","application/json")
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

func setupSDB() {
	SDB = make(map[sdbKey]string)

	SDB[ sdbKey{"a","1"} ] = "./script1.sh"
	SDB[ sdbKey{"a","2"} ] = "./script2.sh"

}

func getEntry(eid string, pol string) (string,bool){
	val, ok := SDB[ sdbKey{eid,pol} ]
	return val,ok
}

//**************************************************************************
//
// Script Call Handlers
//
//**************************************************************************

func callScript( eid string, pol string, msg string) (string, string, error) {
	fmt.Printf("Call string %v, %v, %v",eid,pol,msg)

	dbe,ok := getEntry(eid,pol)
	fmt.Printf(", entry %v, %v\n",dbe,ok)

	cmd := exec.Command(dbe,eid,pol,msg)
	out,_ := cmd.Output()
	//fmt.Printf("\nerror is %v\n",err.Error())
	return "alice", string(out), nil
}