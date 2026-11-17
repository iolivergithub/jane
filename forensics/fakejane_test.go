package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeJane answers the GET endpoints jane_forensics uses, from documents held
// in memory, as janeserver does (lists newest first, ?max, 500 for an item
// that is not there).
type fakeJane struct {
	mu       sync.Mutex
	elements map[string]map[string]any
	intents  map[string]map[string]any
	evs      map[string]map[string]any
	claims   map[string]map[string]any
	results  map[string]map[string]any
	sessions map[string]map[string]any
	log      []map[string]any
	apiKey   string
	gets     map[string]int // path -> count
	methods  map[string]int // anything but GET would be a bug
}

func newFakeJane() *fakeJane {
	return &fakeJane{
		elements: map[string]map[string]any{}, intents: map[string]map[string]any{},
		evs: map[string]map[string]any{}, claims: map[string]map[string]any{},
		results: map[string]map[string]any{}, sessions: map[string]map[string]any{},
		gets: map[string]int{}, methods: map[string]int{},
	}
}

func (f *fakeJane) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.methods[r.Method]++
	if r.Method != http.MethodGet {
		http.Error(w, "only GET here", http.StatusMethodNotAllowed)
		return
	}
	if f.apiKey != "" && r.Header.Get("Authorization") != "Bearer "+f.apiKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.gets[r.URL.Path]++
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	max := int64(200)
	if m, err := strconv.ParseInt(r.URL.Query().Get("max"), 10, 64); err == nil {
		max = m
	}
	one := func(m map[string]map[string]any, id string) {
		if d, ok := m[id]; ok {
			writeJSON(w, d)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]string{"apiversion": "1", "error": "mongo: no documents in result"})
	}
	switch {
	case len(p) == 2 && p[0] == "element":
		one(f.elements, p[1])
	case len(p) == 2 && p[0] == "intent":
		one(f.intents, p[1])
	case len(p) == 2 && p[0] == "expectedValue":
		one(f.evs, p[1])
	case len(p) == 2 && p[0] == "claim":
		one(f.claims, p[1])
	case len(p) == 2 && p[0] == "result":
		one(f.results, p[1])
	case len(p) == 2 && p[0] == "session":
		one(f.sessions, p[1])
	case len(p) == 3 && p[0] == "expectedValues" && p[1] == "element":
		var ids []string
		for id, d := range f.evs {
			if d["elementid"] == p[2] {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		writeJSON(w, map[string]any{"expectedValues": ids, "length": len(ids)})
	case len(p) == 3 && p[0] == "claims" && p[1] == "element":
		ids := newestFirst(f.claims, max, func(d map[string]any) (string, int64) {
			h := d["header"].(map[string]any)
			return h["element"].(map[string]any)["itemid"].(string), h["timing"].(map[string]any)["requested"].(int64)
		}, p[2])
		writeJSON(w, map[string]any{"claims": ids, "length": len(ids)})
	case len(p) == 3 && p[0] == "results" && p[1] == "element":
		ids := newestFirst(f.results, max, func(d map[string]any) (string, int64) {
			return d["elementid"].(string), d["verifiedat"].(int64)
		}, p[2])
		writeJSON(w, map[string]any{"results": ids, "length": len(ids)})
	case len(p) == 1 && p[0] == "pcrnames":
		writeJSON(w, map[string]any{"default": "tcg-pcclient-uefi", "length": 1, "schemes": []any{
			map[string]any{"key": "tcg-pcclient-uefi", "registers": []any{
				map[string]any{"register": "0", "short": "SRTM, firmware code"},
				map[string]any{"register": "7", "short": "Secure Boot policy"},
			}}}})
	case len(p) == 1 && p[0] == "rules":
		writeJSON(w, map[string]any{"length": 2, "rules": []any{
			map[string]any{"name": "tpm2_PCRsMatch", "description": "Checks the PCRs"},
			map[string]any{"name": "tpm2_attestedValue", "description": "Checks the TPM's reported attested value against the expected value"},
		}})
	case len(p) == 1 && p[0] == "log":
		l := f.sortedLog(0)
		if int64(len(l)) > max {
			l = l[:max]
		}
		writeJSON(w, map[string]any{"logentries": l, "length": len(l), "max": max})
	case len(p) == 2 && p[0] == "log" && p[1] == "since":
		d, err := time.ParseDuration(r.URL.Query().Get("duration"))
		if err != nil {
			d = time.Hour
		}
		l := f.sortedLog(time.Now().Add(-d).UnixNano())
		writeJSON(w, map[string]any{"logentries": l, "length": len(l), "duration": r.URL.Query().Get("duration")})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeJane) sortedLog(since int64) []map[string]any {
	var out []map[string]any
	for _, d := range f.log {
		if d["timestamp"].(int64) > since {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["timestamp"].(int64) > out[j]["timestamp"].(int64) })
	return out
}

func newestFirst(m map[string]map[string]any, max int64, key func(map[string]any) (string, int64), eid string) []string {
	type it struct {
		id string
		t  int64
	}
	var items []it
	for id, d := range m {
		if e, t := key(d); e == eid {
			items = append(items, it{id, t})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].t > items[j].t })
	var ids []string
	for i, x := range items {
		if int64(i) >= max {
			break
		}
		ids = append(ids, x.id)
	}
	return ids
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------- the story

// t0 is the start of the test history: a fixed day, so that dates in the
// tests mean something. Hours after t0 are written h(n).
var t0 = time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)

func h(n float64) int64 { return t0.Add(time.Duration(n * float64(time.Hour))).UnixNano() }

func pcrs(p7 string) map[string]any {
	bank := map[string]any{}
	for i := 0; i < 10; i++ {
		bank[strconv.Itoa(i)] = strings.Repeat(fmt.Sprintf("%x", i), 64)
	}
	bank["7"] = p7
	return map[string]any{"sha256": bank}
}

func efivars(sb string, extra bool) map[string]any {
	vars := []any{
		map[string]any{"name": "SecureBoot", "guid": "8be4df61", "value": sb},
		map[string]any{"name": "BootOrder", "guid": "8be4df61", "value": "0001,0002"},
	}
	if extra {
		// a new variable, put first: compared by name, so only an addition
		vars = append([]any{map[string]any{"name": "MokListRT", "guid": "605dab50", "value": "abc"}}, vars...)
	}
	return map[string]any{"efivars": vars, "count": len(vars)}
}

// story makes element E1 with two intents on endpoint tarzan, claims every
// six hours from h(0) to h(66), PCR 7 changing at h(36), SecureBoot at h(48),
// the element updated (description) between h(24) and h(30), the pcrs
// intent's description changed before h(54), and a second element E2 whose
// items must never appear.
func story() *fakeJane {
	f := newFakeJane()
	element := func(desc string, updated int64) map[string]any {
		return map[string]any{
			"itemid": "E1", "name": "testmachine", "description": desc,
			"endpoints":     map[string]any{"tarzan": map[string]any{"endpoint": "http://10.0.0.5:8530", "protocol": "A10HTTPRESTv2"}},
			"tags":          []any{"test"},
			"tpm2":          map[string]any{"ekcerthandle": "0x01C00002"},
			"recordhistory": map[string]any{"created": h(-1), "lastupdated": updated, "reason": "jp update"},
		}
	}
	f.elements["E1"] = element("after update", h(27))
	f.elements["E2"] = map[string]any{"itemid": "E2", "name": "other"}
	intentPCR := func(desc string) map[string]any {
		return map[string]any{"itemid": "IPCR", "name": "std::intent::tpm2::pcrs", "description": desc, "function": "tpm2/pcrs", "parameters": map[string]any{}}
	}
	f.intents["IPCR"] = intentPCR("read the PCRs, v2")
	f.intents["IEFI"] = map[string]any{"itemid": "IEFI", "name": "std::intent::uefi::efivars", "function": "uefi/efivars", "parameters": map[string]any{}}
	ev := func(id, pid string, evs map[string]any, updated int64) map[string]any {
		return map[string]any{"itemid": id, "name": "ev " + id, "elementid": "E1", "intentid": pid, "endpointname": "tarzan", "evs": evs,
			"recordhistory": map[string]any{"created": h(-1), "lastupdated": updated, "reason": "jp update"}}
	}
	goodPCR7 := strings.Repeat("7", 64)
	newPCR7 := strings.Repeat("a", 64)
	f.evs["EVPCR"] = ev("EVPCR", "IPCR", map[string]any{"pcr7": goodPCR7}, h(-1))
	f.evs["EVEFI"] = ev("EVEFI", "IEFI", map[string]any{"secureboot": "1"}, h(50)) // updated after the change, by jp update
	f.evs["EVX"] = map[string]any{"itemid": "EVX", "elementid": "E2", "intentid": "IPCR", "endpointname": "tarzan"}

	n := 0
	for hour := 0.0; hour <= 66; hour += 6 {
		n++
		sid := fmt.Sprintf("S%02d", n)
		el := element("before update", h(-1))
		if hour >= 30 {
			el = element("after update", h(27))
		}
		p7, sb := goodPCR7, "1"
		if hour >= 36 {
			p7 = newPCR7
		}
		if hour >= 48 {
			sb = "0"
		}
		ipcr := intentPCR("read the PCRs")
		if hour >= 54 {
			ipcr = intentPCR("read the PCRs, v2")
		}
		session := map[string]any{"itemid": sid, "message": "jp attest", "timing": map[string]any{"opened": h(hour), "closed": h(hour + 0.1)}}
		var claimIDs, resultIDs []any
		for _, c := range []struct {
			id     string
			intent map[string]any
			body   map[string]any
			ev     string
			rule   string
			ok     bool
		}{
			{fmt.Sprintf("CP%02d", n), ipcr, pcrs(p7), "EVPCR", "tpm2_PCRsMatch", p7 == goodPCR7},
			{fmt.Sprintf("CE%02d", n), f.intents["IEFI"], efivars(sb, hour >= 60), "EVEFI", "uefi_efivarsSecureBoot", sb == "1" || hour >= 54},
		} {
			ts := h(hour) + int64(len(claimIDs))*int64(time.Second)
			f.claims[c.id] = map[string]any{
				"itemid": c.id, "bodytype": "*JSON",
				"header": map[string]any{
					"element": el, "intent": c.intent, "endpointname": "tarzan",
					"endpoint": map[string]any{"endpoint": "http://10.0.0.5:8530", "protocol": "A10HTTPRESTv2"},
					"session":  map[string]any{"itemid": sid},
					"timing":   map[string]any{"requested": ts, "received": ts + int64(300*time.Millisecond)},
				},
				"body": c.body,
			}
			claimIDs = append(claimIDs, c.id)
			rid := "R" + c.id[1:]
			code := int64(0)
			if !c.ok {
				code = 9001
			}
			evcopy := f.evs[c.ev]
			if c.ev == "EVEFI" && hour < 50 {
				evcopy = ev("EVEFI", "IEFI", map[string]any{"secureboot": "1"}, h(-1))
			} else if c.ev == "EVEFI" {
				evcopy = ev("EVEFI", "IEFI", map[string]any{"secureboot": "0"}, h(50))
			}
			f.results[rid] = map[string]any{
				"itemid": rid, "claimid": c.id, "elementid": "E1", "rulename": c.rule,
				"expectedvalue": evcopy, "session": map[string]any{"itemid": sid},
				"verifiedat": ts + int64(time.Second), "result": code, "message": "checked",
			}
			resultIDs = append(resultIDs, rid)
			f.log = append(f.log, map[string]any{"itemid": "L" + c.id, "timestamp": ts, "channel": "claim", "operation": "add", "refid": c.id, "reftype": "claim", "message": "claim added"})
		}
		session["claimlist"], session["resultlist"] = claimIDs, resultIDs
		f.sessions[sid] = session
	}
	// the EFI expected value as it is now, after jp update at h(50)
	f.evs["EVEFI"] = ev("EVEFI", "IEFI", map[string]any{"secureboot": "0"}, h(50))
	// E2's claim and a log entry about it: never in E1's report
	f.claims["CX"] = map[string]any{"itemid": "CX", "header": map[string]any{"element": map[string]any{"itemid": "E2"}, "intent": map[string]any{"itemid": "IPCR"},
		"timing": map[string]any{"requested": h(10)}, "session": map[string]any{"itemid": "SX"}}, "body": map[string]any{}}
	f.log = append(f.log,
		map[string]any{"itemid": "LX", "timestamp": h(10), "channel": "claim", "operation": "add", "refid": "CX", "reftype": "claim", "message": "other"},
		map[string]any{"itemid": "LE1", "timestamp": h(27), "channel": "element", "operation": "update", "refid": "E1", "reftype": "element", "message": "element updated"},
		map[string]any{"itemid": "LI", "timestamp": h(53), "channel": "intent", "operation": "update", "refid": "IPCR", "reftype": "intent", "message": "intent updated"},
	)
	return f
}

// runStory collects and builds the report for E1 against the story.
func runStory(t *testing.T, f *fakeJane, tr timeRange, now time.Time) (*collection, *Report) {
	t.Helper()
	srv := httptest.NewServer(f)
	defer srv.Close()
	cl := newClient(srv.URL, f.apiKey)
	c, err := collect(cl, "E1", tr, defaultMax, now, discard{})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	r := buildReport(c, tr, srv.URL, "jane_forensics test", now, true)
	if f.methods["GET"] == 0 || len(f.methods) != 1 {
		t.Fatalf("jane_forensics must only GET, made %v", f.methods)
	}
	return c, r
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// storyMore adds three more series to the story's element: TPM quotes every
// six hours (new nonce and clock each time; the attested digest changes with
// PCR 7 at h(36), and the reset count goes up then: a reboot), IMA logs (a
// file changes hash at h(42), one is new) and sys/info (the trust agent
// restarts at h(18)).
func storyMore(f *fakeJane) {
	f.intents["IQ"] = map[string]any{"itemid": "IQ", "name": "x86 UEFI SRTM", "function": "tpm2/quote"}
	f.intents["IIMA"] = map[string]any{"itemid": "IIMA", "name": "Linux IMA ASCII Log", "function": "ima/asciilog"}
	f.intents["ISYS"] = map[string]any{"itemid": "ISYS", "name": "System Information", "function": "sys/info"}
	n := 0
	for hour := 0.0; hour <= 66; hour += 6 {
		n++
		// the element copy as the pcrs claim of the same hour has it
		el := f.claims[fmt.Sprintf("CP%02d", n)]["header"].(map[string]any)["element"]
		digest, reset := "aaaa", "3"
		if hour >= 36 {
			digest, reset = "bbbb", "4"
		}
		ima := "10 t1 ima-ng sha256:b00t boot_aggregate\n10 t2 ima-ng sha256:1111 /usr/bin/bash\n10 t3 ima-ng sha256:2222 /usr/lib/x.so\n"
		if hour >= 42 {
			ima = "10 t1 ima-ng sha256:b00t boot_aggregate\n10 t2 ima-ng sha256:9999 /usr/bin/bash\n10 t3 ima-ng sha256:2222 /usr/lib/x.so\n10 t4 ima-ng sha256:3333 /tmp/dropper\n"
		}
		pid := 100
		if hour >= 18 {
			pid = 200
		}
		for _, c := range []struct {
			id, pid string
			body    map[string]any
		}{
			{fmt.Sprintf("CQ%02d", n), "IQ", map[string]any{
				"quote": map[string]any{"magic": "ff544347", "type": "8018", "extradata": fmt.Sprintf("nonce%d", n), "firmwareVersion": "1.2",
					"clockinfo": map[string]any{"clock": fmt.Sprint(1000 * n), "resetcount": reset, "restartcount": "0", "safe": "true"},
					"attested":  map[string]any{"pcrselect": "0,7", "pcrdigest": digest}},
				"signature": map[string]any{"r": fmt.Sprintf("sig%d", n)}, "attest": fmt.Sprintf("att%d", n)}},
			{fmt.Sprintf("CI%02d", n), "IIMA", map[string]any{"asciilog": base64.StdEncoding.EncodeToString([]byte(ima)), "encoded": "base64"}},
			{fmt.Sprintf("CS%02d", n), "ISYS", map[string]any{"os": "linux", "hostname": "testmachine", "pid": pid, "machineid": "m1"}},
		} {
			ts := h(hour) + int64(10*time.Second)
			f.claims[c.id] = map[string]any{
				"itemid": c.id, "bodytype": "*JSON",
				"header": map[string]any{"element": el, "intent": f.intents[c.pid], "endpointname": "tarzan",
					"session": map[string]any{"itemid": "S00"}, "timing": map[string]any{"requested": ts}},
				"body": c.body,
			}
		}
		rid := fmt.Sprintf("RQ%02d", n)
		code := int64(0)
		if digest != "aaaa" {
			code = 9001
		}
		f.results[rid] = map[string]any{"itemid": rid, "claimid": fmt.Sprintf("CQ%02d", n), "elementid": "E1", "rulename": "tpm2_attestedValue",
			"expectedvalue": map[string]any{"itemid": "EVQ", "elementid": "E1", "intentid": "IQ", "endpointname": "tarzan", "evs": map[string]any{"attestedValue": "aaaa"}},
			"parameters":    map[string]any{"pcrscid": fmt.Sprintf("CP%02d", n)},
			"verifiedat":    h(hour) + int64(11*time.Second), "result": code, "message": "attested value " + digest}
	}
	f.sessions["S00"] = map[string]any{"itemid": "S00", "timing": map[string]any{"opened": h(0)}}
}
