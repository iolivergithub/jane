package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fakeStore is an in-memory database with MongoDB's matching for findIn: a
// dotted path goes through arrays, and an array field matches if any element
// does.
type fakeStore struct {
	colls map[string][]bson.Raw
}

func (f *fakeStore) findIn(coll, field string, values []string) ([]bson.Raw, error) {
	want := set(values)
	var out []bson.Raw
	for _, d := range f.colls[coll] {
		for _, v := range pathValues(bson.RawValue{Type: bsontype.EmbeddedDocument, Value: d}, strings.Split(field, ".")) {
			if s, ok := v.StringValueOK(); ok && want[s] {
				out = append(out, d)
				break
			}
		}
	}
	return out, nil
}

func pathValues(v bson.RawValue, parts []string) []bson.RawValue {
	if v.Type == bsontype.Array {
		vals, _ := v.Array().Values()
		var out []bson.RawValue
		for _, e := range vals {
			out = append(out, pathValues(e, parts)...)
		}
		return out
	}
	if len(parts) == 0 {
		return []bson.RawValue{v}
	}
	doc, ok := v.DocumentOK()
	if !ok {
		return nil
	}
	sub, err := doc.LookupErr(parts[0])
	if err != nil {
		return nil
	}
	return pathValues(sub, parts[1:])
}

func (f *fakeStore) deleteIDs(coll string, ids []bson.RawValue) (int64, error) {
	gone := map[string]bool{}
	for _, id := range ids {
		gone[rawKey(id)] = true
	}
	var keep []bson.Raw
	var n int64
	for _, d := range f.colls[coll] {
		if gone[rawKey(d.Lookup("_id"))] {
			n++
		} else {
			keep = append(keep, d)
		}
	}
	f.colls[coll] = keep
	return n, nil
}

func (f *fakeStore) pullRelationships(elementIDs []string, target string) (int64, error) {
	which := set(elementIDs)
	var n int64
	for i, d := range f.colls["elements"] {
		if !which[str(d, "itemid")] {
			continue
		}
		var doc bson.D
		bson.Unmarshal(d, &doc)
		changed := false
		for j, e := range doc {
			if e.Key != "relationships" {
				continue
			}
			var kept bson.A
			for _, r := range e.Value.(bson.A) {
				if r.(bson.D).Map()["itemid"] == target {
					changed = true
				} else {
					kept = append(kept, r)
				}
			}
			doc[j].Value = kept
		}
		if changed {
			f.colls["elements"][i], _ = bson.Marshal(doc)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) itemids(coll string) []string {
	var out []string
	for _, d := range f.colls[coll] {
		out = append(out, str(d, "itemid"))
	}
	sort.Strings(out)
	return out
}

func doc(t *testing.T, d bson.D) bson.Raw {
	t.Helper()
	d = append(bson.D{{"_id", primitive.NewObjectID()}}, d...)
	b, err := bson.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// testDB: element E1, with E2 hosted on it.
//
//	E1: EV1 (intent I1), EV2 (intent I2); claims C1 (I1, session S1) and
//	    C2 (I9, which no longer exists, session S2); results R1 (C1, S1) and
//	    R2 (C2, S2); S4, a profile run for E1 with nothing in it
//	E2: claim C3 and result R3, both in S2, so S2 is shared; S3 has S1 as
//	    parent
//	log: L1 E1, L2 C1, L3 S2, L4 I1, L5 E2, L6 R1
func testDB(t *testing.T) *fakeStore {
	footer := bson.D{{"kid", "k"}, {"alg", "ES256"}, {"hash", []byte{1, 2, 3}}, {"signature", []byte{0, 255}}}
	claim := func(id, eid, intent, sid string) bson.Raw {
		return doc(t, bson.D{{"itemid", id}, {"header", bson.D{
			{"element", bson.D{{"itemid", eid}, {"name", "snapshot"}}},
			{"intent", bson.D{{"itemid", intent}}},
			{"session", bson.D{{"itemid", sid}}},
			{"timing", bson.D{{"requested", int64(1762700000000000000)}}}}},
			{"footer", footer}})
	}
	result := func(id, cid, eid, sid string) bson.Raw {
		return doc(t, bson.D{{"itemid", id}, {"claimid", cid}, {"elementid", eid}, {"session", bson.D{{"itemid", sid}}},
			{"result", int32(0)}, {"footer", footer}})
	}
	session := func(id string, claims, results []string, extra ...bson.E) bson.Raw {
		return doc(t, append(bson.D{{"itemid", id}, {"claimlist", claims}, {"resultlist", results}}, extra...))
	}
	logentry := func(id, ref string) bson.Raw {
		return doc(t, bson.D{{"itemid", id}, {"refid", ref}, {"hash", []byte{9}}})
	}
	return &fakeStore{colls: map[string][]bson.Raw{
		"elements": {
			doc(t, bson.D{{"itemid", "E1"}, {"name", "host1"}}),
			doc(t, bson.D{{"itemid", "E2"}, {"name", "vm1"}, {"relationships", bson.A{
				bson.D{{"relationshiptype", "hosted-on"}, {"itemid", "E1"}},
				bson.D{{"relationshiptype", "runs-on"}, {"itemid", "E9"}}}}}),
		},
		"expectedvalues": {
			doc(t, bson.D{{"itemid", "EV1"}, {"elementid", "E1"}, {"intentid", "I1"}}),
			doc(t, bson.D{{"itemid", "EV2"}, {"elementid", "E1"}, {"intentid", "I2"}}),
			doc(t, bson.D{{"itemid", "EV3"}, {"elementid", "E2"}, {"intentid", "I1"}}),
		},
		"intents": {
			doc(t, bson.D{{"itemid", "I1"}}), doc(t, bson.D{{"itemid", "I2"}}), doc(t, bson.D{{"itemid", "I3"}}),
		},
		"claims":  {claim("C1", "E1", "I1", "S1"), claim("C2", "E1", "I9", "S2"), claim("C3", "E2", "I1", "S2")},
		"results": {result("R1", "C1", "E1", "S1"), result("R2", "C2", "E1", "S2"), result("R3", "C3", "E2", "S2")},
		"sessions": {
			session("S1", []string{"C1"}, []string{"R1"}),
			session("S2", []string{"C2", "C3"}, []string{"R2", "R3"}),
			session("S3", []string{}, []string{}, bson.E{"parent", "S1"}),
			session("S4", []string{}, []string{}, bson.E{"attestationprofilerun", bson.D{{"elementid", "E1"}}}),
		},
		"log": {logentry("L1", "E1"), logentry("L2", "C1"), logentry("L3", "S2"), logentry("L4", "I1"),
			logentry("L5", "E2"), logentry("L6", "R1")},
		"opaqueobjects": {doc(t, bson.D{{"value", "x"}})},
	}}
}

func TestCollect(t *testing.T) {
	c, err := collect(testDB(t), "E1")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"elements":       {"E1"},
		"expectedvalues": {"EV1", "EV2"},
		"intents":        {"I1", "I2"},
		"claims":         {"C1", "C2"},
		"results":        {"R1", "R2"},
		"sessions":       {"S1", "S2", "S4"},
		"log":            {"L1", "L2", "L3", "L6"},
	}
	for coll, ids := range want {
		got := itemIDs(c.docs[coll])
		sort.Strings(got)
		if !reflect.DeepEqual(got, ids) {
			t.Errorf("%s: got %v, want %v", coll, got, ids)
		}
	}
	check := func(name string, got, want []string) {
		t.Helper()
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	check("exclusive", c.exclusiveSessions, []string{"S1", "S4"})
	check("shared", c.sharedSessions, []string{"S2"})
	check("children", c.childSessions, []string{"S3"})
	check("missing intents", c.missingIntents, []string{"I9"})
	if want := []relationshipTo{{"E2", "vm1", "hosted-on", "E1"}}; !reflect.DeepEqual(c.relationshipsTo, want) {
		t.Errorf("relationshipsTo: got %v, want %v", c.relationshipsTo, want)
	}
	removeCounts := map[string]int{}
	for coll, ids := range c.remove {
		removeCounts[coll] = len(ids)
	}
	wantRemove := map[string]int{"results": 2, "claims": 2, "sessions": 2, "log": 3, "expectedvalues": 2, "elements": 1}
	if !reflect.DeepEqual(removeCounts, wantRemove) {
		t.Errorf("remove: got %v, want %v", removeCounts, wantRemove)
	}
}

func TestCollectNotFound(t *testing.T) {
	if _, err := collect(testDB(t), "nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestCollectDuplicateElement(t *testing.T) {
	db := testDB(t)
	db.colls["elements"] = append(db.colls["elements"], doc(t, bson.D{{"itemid", "E1"}}))
	if _, err := collect(db, "E1"); err == nil {
		t.Fatal("two elements with one itemid were accepted")
	}
}

var testSettings = settings{system: "TEST", connection: "mongodb://jane:secret@127.0.0.1:27017", database: "asvr"}

func readBack(t *testing.T, path string) fileContents {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fc fileContents
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	return fc
}

func TestExtractOnly(t *testing.T) {
	db := testDB(t)
	before := testDBCounts(db)
	path := filepath.Join(t.TempDir(), "e1.json")
	if err := extractWith(db, testSettings, cmdline{elementID: "E1", file: path}); err != nil {
		t.Fatal(err)
	}
	if after := testDBCounts(db); !reflect.DeepEqual(before, after) {
		t.Errorf("extract without --delete changed the database: %v -> %v", before, after)
	}
	fc := readBack(t, path)
	md := fc.Metadata
	if md.Operation != "extract" || md.Delete != nil || md.Element.ItemID != "E1" || md.Element.Name != "host1" {
		t.Errorf("metadata: %+v", md)
	}
	if strings.Contains(md.Connection, "secret") {
		t.Errorf("password in the metadata: %s", md.Connection)
	}
	if len(md.ExtractID) != 36 || md.ExtractID[14] != '4' || md.Started == "" || md.Finished == "" {
		t.Errorf("extract ID or times: %q %q %q", md.ExtractID, md.Started, md.Finished)
	}
	if md.Totals.Count != 17 || md.Collections["log"].Count != 4 || md.Collections[relationshipsTo].Count != 1 {
		t.Errorf("counts: %+v", md.Collections)
	}
	for _, coll := range sections {
		if len(fc.Extract[coll]) != md.Collections[coll].Count {
			t.Errorf("%s: %d documents, metadata says %d", coll, len(fc.Extract[coll]), md.Collections[coll].Count)
		}
	}
	if _, ok := fc.Extract["opaqueobjects"]; ok {
		t.Error("opaque objects extracted")
	}
}

// The documents in the file convert back to exactly the BSON in the
// database: binary footers and integer types survive.
func TestRoundTrip(t *testing.T) {
	db := testDB(t)
	path := filepath.Join(t.TempDir(), "e1.json")
	if err := extractWith(db, testSettings, cmdline{elementID: "E1", file: path}); err != nil {
		t.Fatal(err)
	}
	fc := readBack(t, path)
	orig := map[string]bson.Raw{}
	for _, d := range db.colls["claims"] {
		orig[str(d, "itemid")] = d
	}
	for _, j := range fc.Extract["claims"] {
		var b bson.Raw
		if err := bson.UnmarshalExtJSON(j, true, &b); err != nil {
			t.Fatal(err)
		}
		if o := orig[str(b, "itemid")]; string(o) != string(b) {
			t.Errorf("claim %s does not round-trip:\n%s\n%s", str(b, "itemid"), o, b)
		}
	}
}

func TestDelete(t *testing.T) {
	db := testDB(t)
	path := filepath.Join(t.TempDir(), "e1.json")
	if err := extractWith(db, testSettings, cmdline{elementID: "E1", file: path, delete: true}); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"elements":       {"E2"},
		"expectedvalues": {"EV3"},
		"intents":        {"I1", "I2", "I3"}, // never deleted
		"claims":         {"C3"},
		"results":        {"R3"},
		"sessions":       {"S2", "S3"}, // shared, and a child of S1
		"log":            {"L3", "L4", "L5"},
		"opaqueobjects":  {""},
	}
	for coll, ids := range want {
		if got := db.itemids(coll); !reflect.DeepEqual(got, ids) {
			t.Errorf("%s left: got %v, want %v", coll, got, ids)
		}
	}
	rels := docsAt(db.colls["elements"][0], "relationships")
	if len(rels) != 1 || str(rels[0], "itemid") != "E9" {
		t.Errorf("E2's relationships: %v", rels)
	}
	fc := readBack(t, path)
	md := fc.Metadata
	if md.Operation != "extract+delete" || md.Delete == nil || md.Delete.RelationshipsRemoved != 1 ||
		md.Delete.Remove["log"] != 3 || md.Delete.Remove["sessions"] != 2 {
		t.Errorf("metadata: %+v %+v", md, md.Delete)
	}
	var recorded []relationshipTo
	for _, j := range fc.Extract[relationshipsTo] {
		var r relationshipTo
		json.Unmarshal(j, &r)
		recorded = append(recorded, r)
	}
	if !reflect.DeepEqual(recorded, []relationshipTo{{"E2", "vm1", "hosted-on", "E1"}}) {
		t.Errorf("relationshipsTo: %v", recorded)
	}

	// running again: the element is gone
	if err := extractWith(db, testSettings, cmdline{elementID: "E1", file: path + "2", delete: true}); err == nil {
		t.Error("second run found the element")
	}
}

func TestDeleteWithoutRelationships(t *testing.T) {
	db := testDB(t)
	path := filepath.Join(t.TempDir(), "e2.json")
	if err := extractWith(db, testSettings, cmdline{elementID: "E2", file: path, delete: true}); err != nil {
		t.Fatal(err)
	}
	if got := db.itemids("elements"); !reflect.DeepEqual(got, []string{"E1"}) {
		t.Errorf("elements left: %v", got)
	}
	// S2 also holds E1's items, so it stays; E2's own result R3 goes
	if got := db.itemids("sessions"); !reflect.DeepEqual(got, []string{"S1", "S2", "S3", "S4"}) {
		t.Errorf("sessions left: %v", got)
	}
	if got := db.itemids("results"); !reflect.DeepEqual(got, []string{"R1", "R2"}) {
		t.Errorf("results left: %v", got)
	}
}

func TestNeverOverwrites(t *testing.T) {
	db := testDB(t)
	path := filepath.Join(t.TempDir(), "e1.json")
	os.WriteFile(path, []byte("keep"), 0o600)
	before := testDBCounts(db)
	if err := extractWith(db, testSettings, cmdline{elementID: "E1", file: path, delete: true}); err == nil {
		t.Fatal("an existing file was accepted")
	}
	if b, _ := os.ReadFile(path); string(b) != "keep" {
		t.Error("the existing file was changed")
	}
	if after := testDBCounts(db); !reflect.DeepEqual(before, after) {
		t.Errorf("database changed: %v -> %v", before, after)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	db := testDB(t)
	c, err := collect(db, "E1")
	if err != nil {
		t.Fatal(err)
	}
	md := &metadata{ExtractID: "x"}
	enc, err := encodeAll(c, md)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "e1.json")
	if err := writeFile(path, md, enc); err != nil {
		t.Fatal(err)
	}
	if err := verifyFile(path, c, md); err != nil {
		t.Fatalf("untouched file: %v", err)
	}
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(raw), `"L6"`, `"L7"`, 1)), 0o600)
	if err := verifyFile(path, c, md); err == nil {
		t.Fatal("a changed file verified")
	}
}

func testDBCounts(db *fakeStore) map[string]int {
	m := map[string]int{}
	for k, v := range db.colls {
		m[k] = len(v)
		for _, d := range v {
			m[k] += len(d) // catches changed documents too
		}
	}
	return m
}

func TestParseArgs(t *testing.T) {
	o, err := parseArgs([]string{"--delete", "E1", "--config=/x.yaml", "out.json", "--server", "http://j:8520"})
	if err != nil {
		t.Fatal(err)
	}
	want := cmdline{elementID: "E1", file: "out.json", delete: true, config: "/x.yaml", server: "http://j:8520"}
	if o != want {
		t.Errorf("got %+v", o)
	}
	for _, bad := range [][]string{{"E1"}, {"E1", "f", "g"}, {"--delete", "--force", "E1", "f"}, {"--bogus", "E1", "f"}, {"E1", "f", "--config"}} {
		if _, err := parseArgs(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if _, err := parseArgs([]string{"-h"}); err != errHelp {
		t.Errorf("-h: %v", err)
	}
}

func TestLoadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(path, []byte("system:\n  name: J1\ndatabase:\n  connection: mongodb://db:27017\n  name: asvr\nrest:\n  port: 8520\n  listenon: 0.0.0.0\nweb:\n  port: 8540\n  listenon: 10.0.0.5\nx3270:\n  port: 3270\n"), 0o600)
	s, err := loadSettings(cmdline{config: path, database: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if s.system != "J1" || s.connection != "mongodb://db:27017" || s.database != "other" ||
		!reflect.DeepEqual(s.listens, []string{"127.0.0.1:8520", "10.0.0.5:8540", "127.0.0.1:3270"}) {
		t.Errorf("got %+v", s)
	}
	if _, err := loadSettings(cmdline{config: filepath.Join(t.TempDir(), "none.yaml")}); err == nil {
		t.Error("a missing configuration file was accepted")
	}
}

func TestRedactConnection(t *testing.T) {
	if got := redactConnection("mongodb://jane:secret@db:27017/?authSource=admin"); got != "mongodb://jane@db:27017/?authSource=admin" {
		t.Errorf("got %s", got)
	}
}

func TestJaneRunning(t *testing.T) {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		t.Skip("systemd host: the result depends on whether jane runs here")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	busy := l.Addr().String()
	if why, _ := janeRunning(settings{listens: []string{busy}}, ""); !strings.Contains(why, busy) {
		t.Errorf("a listening port: %q", why)
	}
	if why, _ := janeRunning(settings{}, "http://"+busy); !strings.Contains(why, busy) {
		t.Errorf("a listening --server: %q", why)
	}
	if why, _ := janeRunning(settings{}, ""); !strings.Contains(why, "cannot tell") {
		t.Errorf("nothing to check: %q", why)
	}
	l.Close()
	if why, err := janeRunning(settings{listens: []string{busy}}, ""); why != "" || err != nil {
		t.Errorf("a closed port: %q %v", why, err)
	}
}

func TestConfirm(t *testing.T) {
	for in, want := range map[string]bool{
		"yes\n":        true,
		"\n\n yes \n":  true, // empty lines ask again
		"\r\nyes\r\n":  true,
		"y\n":          false,
		"Yes\n":        false,
		"no\n":         false,
		"yesplease\n":  false,
		"":             false, // end of input
		"\n":           false,
		"maybe\nyes\n": false, // the first answer counts
	} {
		var out strings.Builder
		if got := confirm(strings.NewReader(in), &out, "Jane is still running, continue? [yes/no] "); got != want {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
		if !strings.HasPrefix(out.String(), "Jane is still running, continue? [yes/no] ") {
			t.Errorf("%q: asked %q", in, out.String())
		}
	}
	var out strings.Builder
	confirm(strings.NewReader("\n\nno\n"), &out, "Q? ")
	if strings.Count(out.String(), "Q? ") != 3 {
		t.Errorf("empty answers did not ask again: %q", out.String())
	}
}

func TestServerAddress(t *testing.T) {
	for in, want := range map[string]string{"http://jane:8520": "jane:8520", "https://jane": "jane:443", "http://jane": "jane:80"} {
		if got, err := serverAddress(in); err != nil || got != want {
			t.Errorf("%s: got %s %v", in, got, err)
		}
	}
	if _, err := serverAddress("jane"); err == nil {
		t.Error("jane accepted as a URL")
	}
}
