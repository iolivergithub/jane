package restapi

import (
	"fmt"
	"net/http"
)

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
