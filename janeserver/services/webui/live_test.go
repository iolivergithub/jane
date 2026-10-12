package webui

import (
	"testing"
	"time"

	"a10/structures"
)

func TestLiveMinutes(t *testing.T) {
	base := time.Date(2026, 10, 12, 16, 0, 0, 0, time.UTC).UnixNano()
	at := func(min, sec int) structures.Timestamp {
		return structures.Timestamp(base + int64(min)*int64(time.Minute) + int64(sec)*int64(time.Second))
	}
	entries := []structures.LogEntry{ // newest first, as the database returns them
		{Timestamp: at(2, 5), Channel: "R", Operation: "add", Message: "9010"}, // other
		{Timestamp: at(2, 1), Channel: "S", Operation: "update", Message: "result,x"},
		{Timestamp: at(0, 59), Channel: "R", Operation: "add", Message: "9001"}, // fail
		{Timestamp: at(0, 30), Channel: "R", Operation: "add", Message: "0"},    // pass
		{Timestamp: at(0, 20), Channel: "R", Operation: "add", Message: "0"},    // pass
		{Timestamp: at(0, 10), Channel: "C", Operation: "add"},                  // attestation
		{Timestamp: at(0, 0), Channel: "C", Operation: "add"},                   // attestation
		{Timestamp: at(0, 0), Channel: "IM", Operation: "add", RefType: "element"},
	}
	got := liveMinutes(entries)
	m0 := base / int64(time.Minute)
	want := []liveMinute{
		{Minute: m0, Entries: 6, Attestations: 2, Pass: 2, Fail: 1},
		{Minute: m0 + 2, Entries: 2, Other: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d minutes %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("minute %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(liveMinutes(nil)) != 0 {
		t.Error("no entries should give no minutes")
	}
}
