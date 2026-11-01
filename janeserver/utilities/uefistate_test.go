package utilities

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestMeasuredUEFIState(t *testing.T) {
	b, err := os.ReadFile("testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Fatal(err)
	}
	el, err := DecodeEventLog(base64.StdEncoding.EncodeToString(b))
	if err != nil {
		t.Fatal(err)
	}
	st, err := MeasuredUEFIState(el)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		EVS map[string]interface{} `json:"evs"`
	}
	g, _ := os.ReadFile("testdata/cos-85-uefistate.json")
	if err := json.Unmarshal(g, &golden); err != nil {
		t.Fatal(err)
	}
	// compare as JSON, as an expected value is stored
	var got map[string]interface{}
	j, _ := json.Marshal(st.EVS())
	json.Unmarshal(j, &got)
	if !reflect.DeepEqual(got, golden.EVS) {
		t.Errorf("got %v\nwant %v", got, golden.EVS)
	}
	if len(st.Data["SecureBoot"]) != 1 || st.Data["SecureBoot"][0] != 1 || st.Bank != "sha256" {
		t.Errorf("SecureBoot data %v, bank %s", st.Data["SecureBoot"], st.Bank)
	}
}

func TestSCRTMVersion(t *testing.T) {
	for in, want := range map[string]string{
		"G\x00C\x00E\x00\x00\x00": "GCE",
		"1.02\x00":                "1.02",
		"\x01\x02\x03":            "hex:010203",
	} {
		if got := SCRTMVersion([]byte(in)); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
