package main

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"
)

func summaryText(cs *ChangeSet) string {
	if cs == nil {
		return ""
	}
	var ls []string
	for _, l := range cs.Summary {
		ls = append(ls, l.Text)
	}
	return strings.Join(ls, "\n")
}

func TestQuoteIMAAndSysInfo(t *testing.T) {
	f := story()
	storyMore(f)
	_, r := runStory(t, f, timeRange{All: true}, time.Now())

	// every quote has a new nonce, clock and signature: volatile, not a change
	e := findEvent(r, "claim:CQ02")
	body := changeSet(e, "body")
	if body == nil || len(body.Diffs) == 0 {
		t.Fatalf("CQ02: the nonce, clock and signature differ: %+v", e)
	}
	for _, d := range body.Diffs {
		if !d.V {
			t.Errorf("CQ02: %s should be volatile", d.Path)
		}
	}
	if changeSet(e, "decoded") != nil {
		t.Errorf("CQ02: nothing but volatile values changed")
	}
	if r.Counts["eventswithchanges"] != 0 && hasStableChange(e) {
		t.Errorf("CQ02 counted as changed")
	}
	// h(36): new attested digest and a reboot
	dec := summaryText(changeSet(findEvent(r, "claim:CQ07"), "decoded"))
	if !strings.Contains(dec, "Attested PCR digest") || !strings.Contains(dec, "aaaa → bbbb") || !strings.Contains(dec, "TPM reset count") || !strings.Contains(dec, "3 → 4") {
		t.Errorf("CQ07 decoded summary:\n%s", dec)
	}
	// the result against it: outcome and rule message, rule description, parameter link
	res := findEvent(r, "result:RQ07")
	if s := summaryText(changeSet(res, "outcome")); !strings.Contains(s, "0 Success → 9001 Fail") || !strings.Contains(s, "attested value bbbb") {
		t.Errorf("RQ07 outcome summary:\n%s", s)
	}
	if res.RuleDesc == "" || !hasRef(res, "claim:CP07") || res.Prev == nil || res.Prev.ID != "result:RQ06" {
		t.Errorf("RQ07: desc %q refs %+v prev %+v", res.RuleDesc, res.Refs, res.Prev)
	}

	// IMA at h(42): bash has a new hash, /tmp/dropper is new
	ima := summaryText(changeSet(findEvent(r, "claim:CI08"), "decoded"))
	if !strings.Contains(ima, "1 file measured with a different hash: /usr/bin/bash") || !strings.Contains(ima, "1 file measured for the first time: /tmp/dropper") || !strings.Contains(ima, "Entries in the IMA log: 3 → 4") {
		t.Errorf("CI08 IMA summary:\n%s", ima)
	}
	if r.Documents["decoded:claim:CI08"] == "" || !strings.Contains(r.Documents["decoded:claim:CI08"], "/tmp/dropper") {
		t.Errorf("decoded IMA view not in the documents")
	}
	// sys/info at h(18): the agent restarted
	if s := summaryText(changeSet(findEvent(r, "claim:CS04"), "body")); !strings.Contains(s, "restarted") || !strings.Contains(s, "100 → 200") {
		t.Errorf("CS04 summary:\n%s", s)
	}
	// PCR names from Jane's naming scheme
	if s := summaryText(changeSet(findEvent(r, "claim:CP07"), "body")); !strings.Contains(s, "PCR 7 (Secure Boot policy) in the SHA256 bank") {
		t.Errorf("CP07 summary:\n%s", s)
	}
	// UEFI variables decoded as the UEFI_ rules see them (no rawvalue: from text)
	if s := summaryText(changeSet(findEvent(r, "claim:CE09"), "decoded")); !strings.Contains(s, "UEFI variable SecureBoot (Secure Boot enabled state)") {
		t.Errorf("CE09 summary:\n%s", s)
	}
	// element and expected value changes read as such
	if s := summaryText(changeSet(findEvent(r, "claim:CP06"), "element")); !strings.Contains(s, "Element description: before update → after update") || !strings.Contains(s, "Element lastupdated") {
		t.Errorf("CP06 element summary:\n%s", s)
	}
	if s := summaryText(changeSet(findEvent(r, "result:RE10"), "expectedvalue")); !strings.Contains(s, "Secure Boot state") || !strings.Contains(s, ": 1 → 0") {
		t.Errorf("RE10 EV summary:\n%s", s)
	}
	// claims know the previous claim of the same kind
	if p := findEvent(r, "claim:CI08").Prev; p == nil || p.ID != "claim:CI07" || !strings.Contains(p.Label, "Linux IMA ASCII Log") {
		t.Errorf("CI08 prev: %+v", p)
	}
}

func hasStableChange(e *Event) bool {
	for _, cs := range e.Changes {
		for _, d := range cs.Diffs {
			if !d.V {
				return true
			}
		}
	}
	return false
}

func TestEventLogView(t *testing.T) {
	raw, err := os.ReadFile("../lib/uefi/testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Skip("no test event log:", err)
	}
	v := eventLogView(base64.StdEncoding.EncodeToString(raw))
	if v["error"] != nil {
		t.Fatalf("event log not decoded: %v", v["error"])
	}
	for _, k := range []string{"secureboot", "dbx", "events", "eventsperpcr"} {
		if v[k] == nil {
			t.Errorf("decoded event log has no %s (%v)", k, v["stateerror"])
		}
	}
	if len(diffValues(v, eventLogView(base64.StdEncoding.EncodeToString(raw)))) != 0 {
		t.Errorf("the same log differs from itself")
	}
	// a changed dbx and a changed boot application read as what they are
	w := map[string]any{}
	for k, x := range v {
		w[k] = x
	}
	w["dbx"] = "sha256:0000"
	ev := map[string]any{}
	for k, x := range v["events"].(map[string]any) {
		ev[k] = x
	}
	var key string
	for k, x := range ev {
		if strings.HasPrefix(k, "PCR04") && strings.HasPrefix(x.(string), "EFI Boot Services Application") {
			key = k
		}
	}
	if key == "" {
		t.Fatalf("no PCR 4 boot application event in the test log")
	}
	ev[key] = "EFI Boot Services Application | 00"
	w["events"] = ev
	s := strings.Join(func() []string {
		var out []string
		for _, l := range summarize("decoded", diffValues(v, w), sumCtx{function: "uefi/eventlog"}) {
			out = append(out, l.Text)
		}
		return out
	}(), "\n")
	if !strings.Contains(s, "Measured DBX (uefi_logdbx)") || !strings.Contains(s, "PCR 04: 1 event changed (EFI Boot Services Application)") {
		t.Errorf("event log summary:\n%s", s)
	}
}

func TestEfivarsView(t *testing.T) {
	v := efivarsView([]any{
		map[string]any{"name": "SecureBoot", "guid": "g", "rawvalue": base64.StdEncoding.EncodeToString([]byte{1})},
		map[string]any{"name": "db", "guid": "g", "rawvalue": base64.StdEncoding.EncodeToString(make([]byte, 100))},
		map[string]any{"name": "Boot0001", "guid": "g", "errorvalue": "permission denied"},
	})["variables"].(map[string]any)
	if v["SecureBoot"] != "01" || !strings.HasPrefix(v["db"].(string), "sha256:") || v["Boot0001"] != "error: permission denied" {
		t.Errorf("efivars view: %v", v)
	}
}
