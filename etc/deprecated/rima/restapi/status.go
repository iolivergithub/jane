package restapi

import (
	"encoding/json"
	"net/http"

	"rima/configuration"
)

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	response := configuration.ConfigData

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
