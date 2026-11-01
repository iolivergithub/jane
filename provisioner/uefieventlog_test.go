package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// jp's uefieventlog values equal janeserver's for the same log (#134).
func TestUEFIEventLogEVsMatchJane(t *testing.T) {
	b, err := os.ReadFile("../janeserver/utilities/testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"eventlog": base64.StdEncoding.EncodeToString(b)})
	evs, err := uefiEventLogEVs(body)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		EVS map[string]any `json:"evs"`
	}
	g, _ := os.ReadFile("../janeserver/utilities/testdata/cos-85-uefistate.json")
	if err := json.Unmarshal(g, &golden); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	j, _ := json.Marshal(evs)
	json.Unmarshal(j, &got)
	if !reflect.DeepEqual(got, golden.EVS) {
		t.Errorf("got %v\nwant %v", got, golden.EVS)
	}
	if _, err := uefiEventLogEVs([]byte(`{"eventlog":""}`)); err == nil {
		t.Error("an empty log gives an expected value")
	}
}
