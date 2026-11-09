package main

import (
	"fmt"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// collections, in the order they appear in the file.
var collections = []string{"elements", "expectedvalues", "intents", "claims", "results", "sessions", "log"}

// incomingRelationship is a relationship another element has to the
// extracted one.
type incomingRelationship struct {
	ElementID        string `json:"elementid"`
	Name             string `json:"name"`
	RelationshipType string `json:"relationshiptype"`
}

// closure is everything that belongs to one element.
type closure struct {
	elementID   string
	elementName string
	docs        map[string][]bson.Raw // by collection

	exclusiveSessions []string // hold only this element's claims and results: deleted with it
	sharedSessions    []string // also hold other elements' items: extracted, never deleted
	childSessions     []string // not extracted, but their parent is an exclusive session
	missingIntents    []string // referenced, but not in the database
	incoming          []incomingRelationship

	// what --delete removes: _id values by collection, elements last
	remove map[string][]bson.RawValue
}

// collect finds the element eid and everything that belongs to it:
//
//   - expected values with elementid = eid
//   - claims with header.element.itemid = eid
//   - results with elementid = eid, or whose claimid is one of those claims
//   - intents named by those expected values (intentid) and claims
//     (header.intent.itemid): the current documents
//   - sessions holding any of those claims or results, named in their
//     session copies, or opened by an attestation profile run of eid
//   - log entries whose refid is the element or any of the above, except
//     intents (which are shared)
//
// Sessions are exclusive when every claim and result they list belongs to
// eid, otherwise shared. Elements with a relationship to eid are found, but
// not extracted.
func collect(s store, eid string) (*closure, error) {
	c := &closure{elementID: eid, docs: map[string][]bson.Raw{}, remove: map[string][]bson.RawValue{}}

	els, err := s.findIn("elements", "itemid", []string{eid})
	if err != nil {
		return nil, err
	}
	switch len(els) {
	case 0:
		return nil, fmt.Errorf("element %s not found", eid)
	case 1:
	default:
		return nil, fmt.Errorf("element %s: %d documents have this itemid; refusing", eid, len(els))
	}
	c.elementName = str(els[0], "name")
	c.docs["elements"] = els

	evs, err := s.findIn("expectedvalues", "elementid", []string{eid})
	if err != nil {
		return nil, err
	}
	c.docs["expectedvalues"] = sortDocs(evs)

	claims, err := s.findIn("claims", "header.element.itemid", []string{eid})
	if err != nil {
		return nil, err
	}
	c.docs["claims"] = sortDocs(claims)
	cids := itemIDs(claims)

	results, err := findUnion(s, "results", []query{{"elementid", []string{eid}}, {"claimid", cids}})
	if err != nil {
		return nil, err
	}
	c.docs["results"] = results
	rids := itemIDs(results)

	// intents
	var intentIDs []string
	for _, d := range evs {
		intentIDs = append(intentIDs, str(d, "intentid"))
	}
	for _, d := range claims {
		intentIDs = append(intentIDs, str(d, "header.intent.itemid"))
	}
	intentIDs = uniq(intentIDs)
	intents, err := s.findIn("intents", "itemid", intentIDs)
	if err != nil {
		return nil, err
	}
	c.docs["intents"] = sortDocs(intents)
	c.missingIntents = minus(intentIDs, itemIDs(intents))

	// sessions
	var named []string
	for _, d := range claims {
		named = append(named, str(d, "header.session.itemid"))
	}
	for _, d := range results {
		named = append(named, str(d, "session.itemid"))
	}
	sessions, err := findUnion(s, "sessions", []query{
		{"itemid", uniq(named)},
		{"claimlist", cids},
		{"resultlist", rids},
		{"attestationprofilerun.elementid", []string{eid}},
	})
	if err != nil {
		return nil, err
	}
	c.docs["sessions"] = sessions
	cset, rset := set(cids), set(rids)
	var exclusiveDocs []bson.Raw
	for _, d := range sessions {
		run := str(d, "attestationprofilerun.elementid")
		if subset(strs(d, "claimlist"), cset) && subset(strs(d, "resultlist"), rset) && (run == "" || run == eid) {
			c.exclusiveSessions = append(c.exclusiveSessions, str(d, "itemid"))
			exclusiveDocs = append(exclusiveDocs, d)
		} else {
			c.sharedSessions = append(c.sharedSessions, str(d, "itemid"))
		}
	}
	children, err := s.findIn("sessions", "parent", c.exclusiveSessions)
	if err != nil {
		return nil, err
	}
	c.childSessions = minus(itemIDs(children), itemIDs(sessions))

	// log entries
	evids := itemIDs(evs)
	sids := itemIDs(sessions)
	refs := uniq(concat([]string{eid}, evids, cids, rids, sids))
	logs, err := s.findIn("log", "refid", refs)
	if err != nil {
		return nil, err
	}
	c.docs["log"] = sortDocs(logs)

	// relationships of other elements to this one
	related, err := s.findIn("elements", "relationships.itemid", []string{eid})
	if err != nil {
		return nil, err
	}
	for _, d := range sortDocs(related) {
		other := str(d, "itemid")
		if other == eid {
			continue
		}
		for _, r := range docsAt(d, "relationships") {
			if str(r, "itemid") == eid {
				c.incoming = append(c.incoming, incomingRelationship{other, str(d, "name"), str(r, "relationshiptype")})
			}
		}
	}

	// what --delete removes; intents and shared sessions stay
	deleted := set(concat([]string{eid}, evids, cids, rids, c.exclusiveSessions))
	var logDocs []bson.Raw
	for _, d := range logs {
		if deleted[str(d, "refid")] {
			logDocs = append(logDocs, d)
		}
	}
	c.remove["results"] = ids(results)
	c.remove["claims"] = ids(claims)
	c.remove["sessions"] = ids(exclusiveDocs)
	c.remove["log"] = ids(logDocs)
	c.remove["expectedvalues"] = ids(evs)
	c.remove["elements"] = ids(els)
	return c, nil
}

// deleteOrder: a crash part way leaves the element itself visible, and a
// second run picks up what is left.
var deleteOrder = []string{"results", "claims", "sessions", "log", "expectedvalues", "elements"}

// incomingElementIDs are the other elements with a relationship to this one.
func (c *closure) incomingElementIDs() []string {
	var out []string
	for _, r := range c.incoming {
		out = append(out, r.ElementID)
	}
	return uniq(out)
}

type query struct {
	field  string
	values []string
}

// findUnion runs several queries on one collection and merges the results,
// each document once.
func findUnion(s store, coll string, qs []query) ([]bson.Raw, error) {
	seen := map[string]bool{}
	var out []bson.Raw
	for _, q := range qs {
		docs, err := s.findIn(coll, q.field, uniq(q.values))
		if err != nil {
			return nil, err
		}
		for _, d := range docs {
			k := sortKey(d)
			if !seen[k] {
				seen[k] = true
				out = append(out, d)
			}
		}
	}
	return sortDocs(out), nil
}

// sortKey orders documents by _id (an ObjectID begins with its creation
// time, so this is roughly the order they were written), else by itemid.
func sortKey(d bson.Raw) string {
	v, err := d.LookupErr("_id")
	if err != nil {
		return "~" + str(d, "itemid")
	}
	if oid, ok := v.ObjectIDOK(); ok {
		return oid.Hex()
	}
	return string(v.Value)
}

func sortDocs(docs []bson.Raw) []bson.Raw {
	sort.SliceStable(docs, func(i, j int) bool { return sortKey(docs[i]) < sortKey(docs[j]) })
	return docs
}

// str is the string at a dotted path, or "".
func str(d bson.Raw, path string) string {
	v, err := d.LookupErr(strings.Split(path, ".")...)
	if err != nil {
		return ""
	}
	s, _ := v.StringValueOK()
	return s
}

// strs is the string array at key (non-strings skipped).
func strs(d bson.Raw, key string) []string {
	var out []string
	for _, v := range arrayAt(d, key) {
		if s, ok := v.StringValueOK(); ok {
			out = append(out, s)
		}
	}
	return out
}

// docsAt is the documents in the array at key.
func docsAt(d bson.Raw, key string) []bson.Raw {
	var out []bson.Raw
	for _, v := range arrayAt(d, key) {
		if sub, ok := v.DocumentOK(); ok {
			out = append(out, sub)
		}
	}
	return out
}

func arrayAt(d bson.Raw, key string) []bson.RawValue {
	v, err := d.LookupErr(key)
	if err != nil || v.Type != bsontype.Array {
		return nil
	}
	vals, err := v.Array().Values()
	if err != nil {
		return nil
	}
	return vals
}

func itemIDs(docs []bson.Raw) []string {
	var out []string
	for _, d := range docs {
		out = append(out, str(d, "itemid"))
	}
	return uniq(out)
}

func ids(docs []bson.Raw) []bson.RawValue {
	var out []bson.RawValue
	for _, d := range docs {
		if v, err := d.LookupErr("_id"); err == nil {
			out = append(out, v)
		}
	}
	return out
}

// uniq drops empty strings and repeats, keeping the first order.
func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func set(in []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range in {
		m[s] = true
	}
	return m
}

func subset(in []string, of map[string]bool) bool {
	for _, s := range in {
		if !of[s] {
			return false
		}
	}
	return true
}

func minus(a, b []string) []string {
	bs := set(b)
	var out []string
	for _, s := range a {
		if !bs[s] {
			out = append(out, s)
		}
	}
	return out
}

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
