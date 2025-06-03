package restapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"rima/configuration"
	"rima/processing"
)

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

func PcallHandler(w http.ResponseWriter, r *http.Request) {
	var data pcallRequest

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	sid, out, err := processing.CallScript(configuration.ConfigData.JaneURL, data.EID, data.EPN, data.Pol, data.Msg)

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
