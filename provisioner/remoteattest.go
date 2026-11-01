package main

// jp remoteattest (#131): ask Jane to run the element's stored attestation
// profile (POST /element/<eid>/runprov[/<name>]) and print the report. jp
// waits for the answer; Jane runs the profile and the related elements it
// follows. Needs no TPM and no root. See docs/provisioning.md and docs/jp.md.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// remoteTimeout bounds the wait for Jane's report (decision 17).
const remoteTimeout = 10 * time.Minute

// remoteOptions are the remoteattest-only flags.
type remoteOptions struct {
	set         bool // any of them was given
	server      string
	parent      string
	followLimit *string
	noFollow    bool
	parallel    bool
}

// remoteRequest is a checked remoteattest command line.
type remoteRequest struct {
	pfile, profile, server, parent string
	followLimit                    int // -1: Jane's default
	parallel                       bool
}

// parseRemoteArgs reads remoteattest's arguments, [pfile] [profile]: with
// one, it is the provisioning file if such a file exists, else the profile
// name. The server is --server, else JANE_SERVER, else the provisioning
// file's attestationserver (the file must then be given; it is read later).
func parseRemoteArgs(args []string, ro remoteOptions, getenv func(string) string, isFile func(string) bool) (remoteRequest, error) {
	r := remoteRequest{server: ro.server, parent: ro.parent, followLimit: -1, parallel: ro.parallel}
	switch len(args) {
	case 0:
	case 1:
		if isFile(args[0]) {
			r.pfile = args[0]
		} else {
			r.profile = args[0]
		}
	case 2:
		r.pfile, r.profile = args[0], args[1]
	default:
		return r, errors.New("remoteattest takes at most a provisioning file and a profile name")
	}
	if r.profile != "" && !validProfileName(r.profile) {
		return r, fmt.Errorf("%q is not a valid profile name (letters, digits, _ and -, at most 64 characters)", r.profile)
	}
	if ro.noFollow && ro.followLimit != nil {
		return r, errors.New("give --nofollow or --followlimit, not both")
	}
	if ro.noFollow {
		r.followLimit = 0
	}
	if ro.followLimit != nil {
		n, err := strconv.Atoi(*ro.followLimit)
		if err != nil || n < 0 {
			return r, errors.New("--followlimit needs a number, 0 or more")
		}
		r.followLimit = n
	}
	if r.server == "" {
		r.server = getenv("JANE_SERVER")
	}
	if r.server == "" && r.pfile == "" {
		return r, errors.New("Jane's address is needed: give --server, set JANE_SERVER, or give a provisioning file")
	}
	return r, nil
}

func runRemoteAttest(args []string, ro remoteOptions, keyFile *string) int {
	isFile := func(f string) bool {
		fi, err := os.Stat(f)
		return err == nil && fi.Mode().IsRegular()
	}
	r, err := parseRemoteArgs(args, ro, os.Getenv, isFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jp: %v\n%s", err, usage)
		return exitUsage
	}
	p := &ProvisioningFile{AttestationServer: r.server}
	if r.server == "" {
		if !isFile(r.pfile) {
			fmt.Println("Provisioning file", r.pfile, "does not exist")
			return exitError
		}
		lp, err := loadProvisioningFile(r.pfile)
		if err != nil {
			fmt.Println("Error processing", r.pfile+":", err)
			return exitError
		}
		p = &ProvisioningFile{AttestationServer: lp.AttestationServer}
	}

	pv := NewProvisioner(p, "remoteattest")
	pv.Client.HTTP.Timeout = remoteTimeout
	key, err := apiKey(keyFile, os.Getenv, os.ReadFile)
	if err != nil {
		fmt.Println("jp:", err)
		return exitUsage
	}
	pv.Client.APIKey = key
	if key != "" && insecureRemote(p.AttestationServer) {
		fmt.Println("warning: sending the Jane API key over plain HTTP to", p.AttestationServer)
	}
	if err := pv.idFileUnreadable(); err != nil {
		fmt.Println("Error:", err)
		return exitError
	}
	if !pv.idFileExists() {
		fmt.Println(pv.IDFile, "file is missing for a remoteattest operation; run jp create first")
		return exitError
	}
	eid, err := pv.readID()
	if err != nil {
		fmt.Println("Error:", err)
		return exitError
	}
	return pv.remoteAttest(eid, r)
}

func (pv *Provisioner) remoteAttest(eid string, r remoteRequest) int {
	which := "its default attestation profile"
	if r.profile != "" {
		which = "attestation profile " + r.profile
	}
	fmt.Fprintf(pv.Out, "jp: asking Jane to run %s on element %s; this may take a while. Check Jane's Web UI (Sessions) for its status.\n", which, eid)

	rep, err := pv.Client.RunProfile(eid, r)
	if err != nil {
		fmt.Fprintln(pv.Out, "Error:", err)
		var ue *url.Error
		if errors.As(err, &ue) && ue.Timeout() {
			fmt.Fprintln(pv.Out, "jp stopped waiting; Jane may still be running it: check Jane's Web UI (Sessions)")
		}
		return exitError
	}
	rep.print(pv.Out, "")
	s := rep.summary()
	verdict := "PASSED"
	if !s.trusted() {
		verdict = "FAILED"
	}
	fmt.Fprintf(pv.Out, "\nRemote attestation %s: %d rules passed, %d failed or not run, %d intents could not be attested, %d follow errors, %d sessions\n",
		verdict, s.passed, s.failed, s.stepErrors, s.followErrors, s.sessions)
	if !s.trusted() {
		return exitNotTrust
	}
	return exitOK
}

// RemoteReport is Jane's report of a profile run (janeserver
// operations.ProfileRunReport).
type RemoteReport struct {
	ElementID   string             `json:"elementid"`
	ElementName string             `json:"elementname"`
	Profile     string             `json:"profile"`
	Edited      bool               `json:"edited"`
	SessionID   string             `json:"sessionid"`
	Parent      string             `json:"parent"`
	Error       string             `json:"error"`
	Steps       []RemoteStepReport `json:"steps"`
	Vars        []Var              `json:"vars"`
	Follow      []RemoteFollow     `json:"follow"`
}

type RemoteStepReport struct {
	Intent   string             `json:"intent"`
	Endpoint string             `json:"endpoint"`
	ClaimID  string             `json:"claimid"`
	Error    string             `json:"error"`
	Rules    []RemoteRuleReport `json:"rules"`
}

type RemoteRuleReport struct {
	Rule     string `json:"rule"`
	ResultID string `json:"resultid"`
	Result   *int   `json:"result"`
	Message  string `json:"message"`
	NotRun   string `json:"notrun"`
	Error    string `json:"error"`
}

type RemoteFollow struct {
	Relationship string        `json:"relationship"`
	Target       string        `json:"target"`
	Profile      string        `json:"profile"`
	Kind         string        `json:"kind"` // run, skipped, limit, error
	Message      string        `json:"message"`
	Report       *RemoteReport `json:"report"`
}

// RunProfile asks Jane to run a profile of the element eid and returns its
// report.
func (c *Client) RunProfile(eid string, r remoteRequest) (*RemoteReport, error) {
	path := "/element/" + url.PathEscape(eid) + "/runprov"
	if r.profile != "" {
		path += "/" + url.PathEscape(r.profile)
	}
	q := url.Values{}
	if r.followLimit >= 0 {
		q.Set("followlimit", strconv.Itoa(r.followLimit))
	}
	if r.parallel {
		q.Set("parallel", "true")
	}
	if r.parent != "" {
		q.Set("parent", r.parent)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	status, rb, err := c.do(http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		if noSuchRoute(status, rb) {
			return nil, errNoProfiles
		}
		var m struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(rb, &m) == nil && m.Error != "" {
			return nil, fmt.Errorf("Jane refused the run (status %d): %s", status, m.Error)
		}
		return nil, &ServerError{http.MethodPost, path, status, strings.TrimSpace(string(rb))}
	}
	var rep RemoteReport
	if err := json.Unmarshal(rb, &rep); err != nil {
		return nil, fmt.Errorf("Jane's report could not be read: %w", err)
	}
	return &rep, nil
}

func (rep *RemoteReport) print(w io.Writer, indent string) {
	ed := ""
	if rep.Edited {
		ed = " (edited)"
	}
	fmt.Fprintf(w, "\n%sProfile %s%s on %s (element %s), session %s\n", indent, rep.Profile, ed, rep.ElementName, rep.ElementID, rep.SessionID)
	if rep.Error != "" {
		fmt.Fprintf(w, "%s  ERROR  %s\n", indent, rep.Error)
	}
	for _, st := range rep.Steps {
		fmt.Fprintf(w, "%s  %s on %s", indent, st.Intent, st.Endpoint)
		if st.ClaimID != "" {
			fmt.Fprintf(w, "  (claim %s)", st.ClaimID)
		}
		fmt.Fprintln(w)
		if st.Error != "" {
			fmt.Fprintf(w, "%s    ERROR  attestation failed: %s\n", indent, st.Error)
			continue
		}
		if len(st.Rules) == 0 {
			fmt.Fprintf(w, "%s    -      no rules listed; claim collected only\n", indent)
		}
		for _, rr := range st.Rules {
			switch {
			case rr.NotRun != "":
				fmt.Fprintf(w, "%s    NOTRUN %s: %s\n", indent, rr.Rule, rr.NotRun)
			case rr.Result != nil && *rr.Result == ResultSuccess:
				fmt.Fprintf(w, "%s    PASS   %s\n", indent, rr.Rule)
			case rr.Result != nil:
				fmt.Fprintf(w, "%s    FAIL   %s: %s %s\n", indent, rr.Rule, ResultName(*rr.Result), rr.Message)
			default:
				fmt.Fprintf(w, "%s    FAIL   %s: %s\n", indent, rr.Rule, rr.Error)
			}
		}
	}
	if len(rep.Vars) > 0 {
		fmt.Fprintf(w, "%s  %-20s | %-6s | %s\n", indent, "var", "kind", "value")
		for _, v := range rep.Vars {
			fmt.Fprintf(w, "%s  %-20s | %-6s | %s\n", indent, v.Name, v.Kind, v.Value)
		}
	}
	for _, f := range rep.Follow {
		prof := f.Profile
		if prof == "" {
			prof = "(default)"
		}
		fmt.Fprintf(w, "%s  follow %s -> %s %s: %s %s\n", indent, f.Relationship, f.Target, prof, strings.ToUpper(f.Kind), f.Message)
		if f.Report != nil {
			f.Report.print(w, indent+"    ")
		}
	}
}

type remoteSummary struct {
	passed, failed, stepErrors, followErrors, sessions int
	runErrors                                          int
}

// trusted: every rule in the whole tree ran and passed, every intent was
// attested, every run was made and no follow had an error (decision 25:
// skips and the limit don't count).
func (s remoteSummary) trusted() bool {
	return s.failed == 0 && s.stepErrors == 0 && s.followErrors == 0 && s.runErrors == 0
}

func (rep *RemoteReport) summary() remoteSummary {
	var s remoteSummary
	var walk func(r *RemoteReport)
	walk = func(r *RemoteReport) {
		if r.SessionID != "" {
			s.sessions++
		}
		if r.Error != "" {
			s.runErrors++
		}
		for _, st := range r.Steps {
			if st.Error != "" {
				s.stepErrors++
			}
			for _, rr := range st.Rules {
				if rr.NotRun == "" && rr.Error == "" && rr.Result != nil && *rr.Result == ResultSuccess {
					s.passed++
				} else {
					s.failed++
				}
			}
		}
		for _, f := range r.Follow {
			if f.Kind == "error" {
				s.followErrors++
			}
			if f.Report != nil {
				walk(f.Report)
			}
		}
	}
	walk(rep)
	return s
}
