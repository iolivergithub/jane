package main

// Attestation profiles (#131): jp create and update store the attestation
// part of the provisioning file in Jane as a named profile, which Jane can
// then run (jp remoteattest, REST, the Web UI). See docs/provisioning.md.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/iolivergithub/jane/lib/janeapi"
)

// defaultProfileName is used when the provisioning file names no profile.
const defaultProfileName = "default"

// validProfileName is Jane's check (lib/janeapi): letters, digits, _ and -,
// at most 64.
func validProfileName(n string) bool { return janeapi.ValidProfileName(n) }

// checkProfile checks the profile names in the file before anything runs
// (exit 2 on a mistake, like the variables).
func (p *ProvisioningFile) checkProfile() error {
	if p.DefaultAttestationProfile != "" && !validProfileName(p.DefaultAttestationProfile) {
		return &VarError{fmt.Sprintf("defaultattestationprofile: %q is not a valid profile name (letters, digits, _ and -, at most 64 characters)", p.DefaultAttestationProfile)}
	}
	for _, f := range p.Follow {
		if f.Relationship == "" {
			return &VarError{"follow: every entry needs a relationship, e.g. hosted-on"}
		}
		if f.Profile != "" && !validProfileName(f.Profile) {
			return &VarError{fmt.Sprintf("follow %s: %q is not a valid profile name", f.Relationship, f.Profile)}
		}
	}
	return nil
}

// The profile as Jane stores it (lib/janeapi, shared with janeserver).
type (
	AttestationProfile = janeapi.AttestationProfile
	ProfileStep        = janeapi.ProfileStep
	ProfileRule        = janeapi.ProfileRule
)

// profileName is the name of the profile this file writes.
func (p *ProvisioningFile) profileName() string {
	if p.DefaultAttestationProfile == "" {
		return defaultProfileName
	}
	return p.DefaultAttestationProfile
}

// buildProfile turns the file's evs: (without type: and issuerurl:, which
// only concern expected values) and follow: into a profile. ok is false if
// there is nothing to store.
func (p *ProvisioningFile) buildProfile() (prof AttestationProfile, ok bool) {
	prof.Description = p.AttestationProfileDescription
	for _, ev := range p.EVS {
		st := ProfileStep{Intent: ev.IntentID, Endpoint: ev.Spec.Protocol, CID: ev.Spec.CID}
		for _, r := range ev.Spec.Rules {
			st.Rules = append(st.Rules, ProfileRule{Name: r.Name, RID: r.RID, ResVar: r.ResVar, Parameters: r.Parameters})
		}
		prof.Steps = append(prof.Steps, st)
	}
	prof.Follow = p.Follow
	return prof, len(prof.Steps) > 0 || len(prof.Follow) > 0
}

// errNoProfiles means the Jane server does not know attestation profiles
// (older than 1.6).
var errNoProfiles = errors.New("this Jane does not support attestation profiles (it is older than 1.6)")

// PutProfile stores the profile name on the element eid and makes it the
// element's default.
func (c *Client) PutProfile(eid, name string, prof AttestationProfile) error {
	path := "/element/" + url.PathEscape(eid) + "/attestationprofile/" + url.PathEscape(name)
	status, rb, err := c.do(http.MethodPut, path, prof)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if noSuchRoute(status, rb) {
		return errNoProfiles
	}
	return &ServerError{http.MethodPut, path, status, strings.TrimSpace(string(rb))}
}

// noSuchRoute reports whether Jane answered that it has no such endpoint
// (echo's own 404/405, not one of Jane's error messages).
func noSuchRoute(status int, body []byte) bool {
	if status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		return false
	}
	var m struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	json.Unmarshal(body, &m)
	return m.Error == ""
}

// storeProfile is the last step of create and update: it stores the file's
// profile on the element (decision 2). A file without a profile name gets
// "default" with a warning; one with neither evs nor follow stores nothing.
func (pv *Provisioner) storeProfile() error {
	prof, ok := pv.P.buildProfile()
	if !ok {
		fmt.Fprintln(pv.Out, "WARNING: the provisioning file has neither evs nor follow: no attestation profile stored")
		return nil
	}
	name := pv.P.profileName()
	if pv.P.DefaultAttestationProfile == "" {
		fmt.Fprintln(pv.Out, "WARNING: no defaultattestationprofile in the provisioning file: storing the profile as \""+name+"\"; add e.g. \"defaultattestationprofile: "+name+"\"")
	}
	eid, err := pv.readID()
	if err != nil {
		fmt.Fprintln(pv.Out, "WARNING: no element ID (is processelement in the worklist?): no attestation profile stored")
		return nil
	}
	if err := pv.Client.PutProfile(eid, name, prof); err != nil {
		if errors.Is(err, errNoProfiles) {
			fmt.Fprintf(pv.Out, "WARNING: %v: profile not stored\n", err)
			return nil
		}
		return fmt.Errorf("storing attestation profile %s: %w", name, err)
	}
	fmt.Fprintf(pv.Out, "attestation profile %s stored (%d steps, %d follow entries); it is now the element's default\n", name, len(prof.Steps), len(prof.Follow))
	return nil
}
