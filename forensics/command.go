package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed report.html
var reportTemplate string

// reportPlaceholder in report.html is replaced by the report as JSON.
const reportPlaceholder = "/*JANE_FORENSICS_REPORT*/null"

// forensicsCommand is the whole run: read from Jane, write the HTML file.
func forensicsCommand(o cmdline, tr timeRange, now time.Time) error {
	key, err := apiKey(o.apiKeyFile, os.Getenv, os.ReadFile)
	if err != nil {
		return err
	}
	if key != "" && insecureRemote(o.server) {
		fmt.Fprintln(os.Stderr, "warning: sending the Jane API key over plain HTTP to", o.server)
	}
	out := o.out
	if out == "" {
		out = fmt.Sprintf("jane_forensics_%s_%s.html", safeName(o.elementID), now.UTC().Format("20060102T150405Z"))
	}
	if out != "-" {
		if _, err := os.Stat(out); err == nil {
			return fmt.Errorf("%s exists: jane_forensics never overwrites a file", out)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	cl := newClient(o.server, key)
	fmt.Fprintf(os.Stderr, "jane_forensics: Jane at %s, range %s\n", cl.baseURL, tr)
	c, err := collect(cl, o.elementID, tr, o.max, now, os.Stderr)
	if err != nil {
		return err
	}
	tool := fmt.Sprintf("jane_forensics %s (build %s)", VERSION, BUILD)
	r := buildReport(c, tr, cl.baseURL, tool, now, !o.noDocuments)
	r.Counts["requests"] = cl.calls

	page, err := renderHTML(r)
	if err != nil {
		return err
	}
	if out == "-" {
		_, err = os.Stdout.Write(page)
		return err
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(page); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "jane_forensics: wrote %s (%d events, %d bytes)\n", out, len(r.Events), len(page))
	for _, w := range r.Warnings {
		fmt.Fprintln(os.Stderr, "jane_forensics: warning:", w)
	}
	return nil
}

// renderHTML puts the report into the page. encoding/json escapes <, > and &,
// so nothing in the data can end the <script> element it sits in.
func renderHTML(r *Report) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	data := strings.TrimSpace(buf.String())
	if !strings.Contains(reportTemplate, reportPlaceholder) {
		return nil, fmt.Errorf("report.html has no placeholder for the data")
	}
	page := strings.Replace(reportTemplate, reportPlaceholder, data, 1)
	return []byte(page), nil
}

// safeName keeps letters, digits, - and _ of an itemid, for a file name.
func safeName(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// insecureRemote reports whether the server URL is plain HTTP to a host
// other than this machine, as jp checks.
func insecureRemote(server string) bool {
	u, err := url.Parse(server)
	if err != nil || u.Scheme != "http" {
		return false
	}
	h := u.Hostname()
	return h != "localhost" && h != "127.0.0.1" && h != "::1"
}
