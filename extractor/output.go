package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"

	"go.mongodb.org/mongo-driver/bson"
)

// formatVersion changes if the layout of the file changes.
const formatVersion = 1

const documentFormat = "MongoDB Extended JSON v2, canonical mode; one document per array entry, " +
	"sha256 over each document's bytes as written followed by a newline"

type metadata struct {
	Tool          string `json:"tool"`
	Version       string `json:"version"`
	Build         string `json:"build"`
	FormatVersion int    `json:"formatversion"`
	ExtractID     string `json:"extractid"` // UUID4
	Operation     string `json:"operation"` // extract or extract+delete

	Element struct {
		ItemID string `json:"itemid"`
		Name   string `json:"name"`
	} `json:"element"`

	Started  string `json:"started"`  // UTC, RFC 3339, when reading began
	Finished string `json:"finished"` // when reading ended (before the file was written)

	Host       string `json:"host"`
	Operator   string `json:"operator"`
	System     string `json:"system,omitempty"` // system: name from config.yaml
	Config     string `json:"config,omitempty"`
	Database   string `json:"database"`
	Connection string `json:"connection"` // without any password

	DocumentFormat string                    `json:"documentformat"`
	Collections    map[string]collectionInfo `json:"collections"` // the extract's sections, relationshipsTo included
	Totals         collectionInfo            `json:"totals"`      // sha256 empty

	Sessions struct {
		Exclusive []string `json:"exclusive"` // only this element's claims and results
		Shared    []string `json:"shared"`    // also other elements': never deleted
		Children  []string `json:"children"`  // not extracted; parent is an exclusive session
	} `json:"sessions"`
	MissingIntents []string `json:"missingintents"`

	Delete *deletePlan `json:"delete,omitempty"`
}

type collectionInfo struct {
	Count  int    `json:"count"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
}

// deletePlan is what --delete removes after the file is written.
type deletePlan struct {
	Remove               map[string]int `json:"remove"`               // documents by collection
	RelationshipsRemoved int            `json:"relationshipsremoved"` // entries pulled from other elements
	Kept                 string         `json:"kept"`
}

// encoded is one collection's documents as they go into the file.
type encoded struct {
	docs [][]byte
	info collectionInfo
}

// encodeSection hashes and measures one section's encoded entries.
func encodeSection(entries [][]byte) encoded {
	e := encoded{docs: entries}
	h := sha256.New()
	for _, j := range entries {
		h.Write(j)
		h.Write([]byte("\n"))
		e.info.Bytes += len(j)
	}
	e.info.Count = len(entries)
	e.info.SHA256 = hex.EncodeToString(h.Sum(nil))
	return e
}

func newUUID4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func operatorName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// encodeAll encodes every section and fills in the counts, sizes and
// hashes of md: the collections as canonical Extended JSON, relationshipsTo
// as plain JSON.
func encodeAll(c *closure, md *metadata) (map[string]encoded, error) {
	entries := map[string][][]byte{}
	for _, coll := range collections {
		for _, d := range c.docs[coll] {
			j, err := bson.MarshalExtJSON(d, true, false)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", coll, err)
			}
			entries[coll] = append(entries[coll], j)
		}
	}
	for _, r := range c.relationshipsTo {
		j, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		entries[relationshipsTo] = append(entries[relationshipsTo], j)
	}
	enc := map[string]encoded{}
	md.Collections = map[string]collectionInfo{}
	for _, sec := range sections {
		e := encodeSection(entries[sec])
		enc[sec] = e
		md.Collections[sec] = e.info
		md.Totals.Count += e.info.Count
		md.Totals.Bytes += e.info.Bytes
	}
	return enc, nil
}

// writeFile writes the file, refusing to overwrite one; a partly written
// file is removed.
func writeFile(path string, md *metadata, enc map[string]encoded) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	w := bufio.NewWriter(f)
	mdj, err := json.MarshalIndent(md, "  ", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "{\n  \"metadata\": %s,\n  \"extract\": {\n", mdj)
	for i, coll := range sections {
		fmt.Fprintf(w, "    %q: [", coll)
		for j, d := range enc[coll].docs {
			if j > 0 {
				w.WriteString(",")
			}
			w.WriteString("\n      ")
			w.Write(d)
		}
		if len(enc[coll].docs) > 0 {
			w.WriteString("\n    ")
		}
		w.WriteString("]")
		if i < len(sections)-1 {
			w.WriteString(",")
		}
		w.WriteString("\n")
	}
	w.WriteString("  }\n}\n")
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

// fileContents is the file as read back.
type fileContents struct {
	Metadata metadata                     `json:"metadata"`
	Extract  map[string][]json.RawMessage `json:"extract"`
}

// verifyFile reads the file back and checks that it is what was meant to be
// written: the metadata's counts and hashes match every section, every
// document parses back into BSON, every document --delete would remove is
// in it, and so is every relationship it would remove. Only then may
// anything be deleted.
func verifyFile(path string, c *closure, md *metadata) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fc fileContents
	if err := json.Unmarshal(raw, &fc); err != nil {
		return fmt.Errorf("%s does not parse: %w", path, err)
	}
	if fc.Metadata.ExtractID != md.ExtractID {
		return fmt.Errorf("%s: extract ID %q, expected %q", path, fc.Metadata.ExtractID, md.ExtractID)
	}
	for _, coll := range sections {
		docs := fc.Extract[coll]
		want := md.Collections[coll]
		h := sha256.New()
		got := map[string]bool{}
		for i, d := range docs {
			h.Write(d)
			h.Write([]byte("\n"))
			var b bson.Raw
			if err := bson.UnmarshalExtJSON(d, true, &b); err != nil {
				return fmt.Errorf("%s: %s[%d] does not parse as Extended JSON: %w", path, coll, i, err)
			}
			if v, err := b.LookupErr("_id"); err == nil {
				got[rawKey(v)] = true
			}
		}
		if len(docs) != want.Count || fc.Metadata.Collections[coll].Count != want.Count {
			return fmt.Errorf("%s: %s has %d documents, expected %d", path, coll, len(docs), want.Count)
		}
		if sum := hex.EncodeToString(h.Sum(nil)); sum != want.SHA256 || fc.Metadata.Collections[coll].SHA256 != want.SHA256 {
			return fmt.Errorf("%s: %s hash mismatch", path, coll)
		}
		for _, id := range c.remove[coll] {
			if !got[rawKey(id)] {
				return fmt.Errorf("%s: a %s document to be deleted is not in the file", path, coll)
			}
		}
	}
	var rels []relationshipTo
	for _, j := range fc.Extract[relationshipsTo] {
		var r relationshipTo
		if err := json.Unmarshal(j, &r); err != nil {
			return fmt.Errorf("%s: %s: %w", path, relationshipsTo, err)
		}
		rels = append(rels, r)
	}
	if len(rels) != len(c.relationshipsTo) {
		return fmt.Errorf("%s: %s has %d entries, expected %d", path, relationshipsTo, len(rels), len(c.relationshipsTo))
	}
	for i := range rels {
		if rels[i] != c.relationshipsTo[i] {
			return fmt.Errorf("%s: %s[%d] differs from the database", path, relationshipsTo, i)
		}
	}
	return nil
}

// rawKey identifies a BSON value (type and bytes), for sets of _id values.
func rawKey(v bson.RawValue) string {
	return string(rune(v.Type)) + string(v.Value)
}
