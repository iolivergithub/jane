package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iolivergithub/jane/lib/janeapi"
)

// client reads from Jane's REST API. It only ever sends GET requests.
type client struct {
	baseURL string
	http    *http.Client
	apiKey  string // sent as "Authorization: Bearer <key>" when set (#89)
	calls   int    // requests made, for the report
}

func newClient(base, apiKey string) *client {
	return &client{
		baseURL: strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 60 * time.Second},
		apiKey:  apiKey,
	}
}

var errUnauthorized = errors.New("Jane refused the request with HTTP 401 (API key required, or not accepted); " +
	"set JANE_API_KEY or give --apikeyfile")

var errTooManyFailures = errors.New("Jane refused the request with HTTP 429: too many failed attempts from this address; " +
	"check the API key and try again in a few minutes")

// statusError is a request Jane answered, but not with 200.
type statusError struct {
	path   string
	status int
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("GET %s: status %d: %s", e.path, e.status, e.body)
}

// get GETs path and returns the body of a 200 answer.
func (c *client) get(path string) ([]byte, error) {
	c.calls++
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return b, nil
	case http.StatusUnauthorized:
		return nil, errUnauthorized
	case http.StatusTooManyRequests:
		return nil, errTooManyFailures
	}
	return nil, &statusError{path, resp.StatusCode, strings.TrimSpace(string(b))}
}

// getDoc GETs one document (element, claim, ...), keeping the raw JSON.
func (c *client) getDoc(path string) (*doc, error) {
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	return parseDoc(b)
}

// getList GETs a list of itemids, such as {"claims": [...], "length": n}.
func (c *client) getList(path, key string) ([]string, error) {
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("GET %s: bad answer: %v", path, err)
	}
	var ids []string
	if raw, ok := m[key]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, fmt.Errorf("GET %s: bad %s list: %v", path, key, err)
		}
	}
	return ids, nil
}

// getLogEntries GETs a log listing, {"logentries": [...]}, as documents.
func (c *client) getLogEntries(path string) ([]*doc, error) {
	b, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("GET %s: bad answer: %v", path, err)
	}
	var raws []json.RawMessage
	if raw, ok := m["logentries"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, fmt.Errorf("GET %s: bad log entries: %v", path, err)
		}
	}
	var out []*doc
	for _, r := range raws {
		d, err := parseDoc(r)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %v", path, err)
		}
		out = append(out, d)
	}
	return out, nil
}

func esc(s string) string { return url.PathEscape(s) }

// doc is one document from Jane: decoded (numbers kept exact, since Jane's
// timestamps are nanoseconds and do not fit a float64) and as received.
type doc struct {
	m   map[string]any
	raw []byte
}

func parseDoc(b []byte) (*doc, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("bad document: %v", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return &doc{m: m, raw: b}, nil
}

// get returns the value at a dotted path, or nil.
func (d *doc) get(path string) any {
	if d == nil {
		return nil
	}
	return lookup(d.m, path)
}

func lookup(v any, path string) any {
	for _, k := range strings.Split(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

// str returns the string at path, or "".
func (d *doc) str(path string) string {
	s, _ := d.get(path).(string)
	return s
}

// num returns the integer at path, or 0.
func (d *doc) num(path string) int64 {
	switch n := d.get(path).(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			f, _ := n.Float64()
			return int64(f)
		}
		return i
	}
	return 0
}

// apiKey picks the Jane REST API key, as jp does: the contents of
// --apikeyfile when given, else JANE_API_KEY. An empty result means no key is
// sent. A key that is not "jane_" and 43 base64url characters is refused
// here, as Jane would refuse it.
func apiKey(file *string, getenv func(string) string, readFile func(string) ([]byte, error)) (string, error) {
	key := getenv("JANE_API_KEY")
	from := "JANE_API_KEY"
	if file != nil {
		b, err := readFile(*file)
		if err != nil {
			return "", fmt.Errorf("cannot read the API key file: %v", err)
		}
		key, from = string(b), *file
	}
	key = strings.TrimSpace(key)
	if key != "" && !janeapi.ValidAPIKey(key) {
		return "", fmt.Errorf("%s does not hold a Jane API key (jane_ followed by 43 characters, as printed by addjaneuser --apikey)", from)
	}
	return key, nil
}
