package main

import (
	"errors"
	"fmt"
)

// Exit codes. 0 and 3 are the attestation verdict; 1 and 2 mean jp itself
// could not do its job.
const (
	exitOK       = 0 // done; for attest, every rule passed
	exitError    = 1 // setup, server or TPM error
	exitUsage    = 2 // bad command line, or a mistake in the provisioning file's variables
	exitNotTrust = 3 // attest: a rule did not pass or an intent could not be attested
)

// IntentOutcome is the attestation of one intent in the provisioning file.
type IntentOutcome struct {
	IntentID, Endpoint string
	ClaimID            string
	Err                error // attestation or claim retrieval failed
	Rules              []RuleOutcome
}

type RuleOutcome struct {
	Rule   string
	Result *VerifyResult // nil if the rule was not run
	NotRun string        // why the rule was not run
}

// Passed reports whether the rule ran and succeeded.
func (ro RuleOutcome) Passed() bool { return ro.Result != nil && ro.Result.Passed() }

// Var is a variable set during an attest run (#72): Kind is cid, rid or resvar.
type Var struct {
	Name, Kind, Value string
}

// AttestReport is the outcome of an attest run.
type AttestReport struct {
	ElementID, SessionID string
	Intents              []IntentOutcome
	Vars                 []Var // in the order they were set
}

// setVar records a variable, ignoring an empty name (not requested).
func (r *AttestReport) setVar(vars map[string]string, name, kind, value string) {
	if name == "" {
		return
	}
	vars[name] = value
	r.Vars = append(r.Vars, Var{name, kind, value})
}

// substitute returns params with every value that is exactly "$name"
// replaced by that variable, or the name of a variable that has no value
// because the step that sets it failed.
func substitute(params map[string]any, vars map[string]string) (map[string]any, string) {
	if len(params) == 0 {
		return nil, ""
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		if name, ok := varRef(v); ok {
			val, set := vars[name]
			if !set || val == "" {
				return nil, name
			}
			out[k] = val
			continue
		}
		out[k] = v
	}
	return out, ""
}

// Counts returns the number of rules passed and failed, and the number of
// intents that could not be attested.
func (r *AttestReport) Counts() (passed, failed, intentErrors int) {
	for _, in := range r.Intents {
		if in.Err != nil {
			intentErrors++
		}
		for _, ro := range in.Rules {
			if ro.Passed() {
				passed++
			} else {
				failed++
			}
		}
	}
	return
}

// Trusted reports whether every intent was attested and every rule passed.
func (r *AttestReport) Trusted() bool {
	_, failed, errs := r.Counts()
	return failed == 0 && errs == 0
}

// Attest attests the already-registered element (its ID read from the
// element ID file) against every intent in the provisioning file's evs list
// and verifies every rule listed for it, in one session. It does not touch
// the TPM, the element or its expected values, and ignores provisionworklist.
//
// An intent that cannot be attested, or a rule that does not pass, is
// recorded in the report and the run carries on. An error is returned only
// if the Jane server cannot be reached or the session cannot be opened.
func (pv *Provisioner) Attest() (report *AttestReport, err error) {
	eid, err := pv.readID()
	if err != nil {
		return nil, err
	}
	if len(pv.P.EVS) == 0 {
		return nil, errors.New("the provisioning file lists no intents under evs")
	}

	sid, err := pv.Client.OpenSession("Attestation Session")
	if err != nil {
		return nil, err
	}
	report = &AttestReport{ElementID: eid, SessionID: sid}
	defer func() {
		if cerr := pv.Client.CloseSession(sid); err == nil {
			err = cerr
		}
	}()

	var srvErr *ServerError
	vars := map[string]string{}
	for _, ev := range pv.P.EVS {
		in := IntentOutcome{IntentID: ev.IntentID, Endpoint: ev.Spec.Protocol}
		fmt.Fprintln(pv.Out, "attesting", ev.IntentID, "on", ev.Spec.Protocol)

		cid, err := pv.Client.Attest(eid, in.IntentID, in.Endpoint, sid)
		if err == nil {
			in.ClaimID = cid
			report.setVar(vars, ev.Spec.CID, "cid", cid)
			// The claim is fetched to check the attestation produced one;
			// its content is judged by the rules.
			_, err = pv.Client.GetClaim(cid)
		}
		if err != nil {
			if !errors.As(err, &srvErr) {
				return report, err
			}
			in.Err = err
			report.Intents = append(report.Intents, in)
			continue
		}

		for _, rule := range ev.Spec.Rules {
			params, unset := substitute(rule.Parameters, vars)
			if unset != "" {
				in.Rules = append(in.Rules, RuleOutcome{Rule: rule.Name, NotRun: "$" + unset + " has no value (its step failed)"})
				continue
			}
			v, err := pv.Client.Verify(cid, sid, rule.Name, params)
			if err != nil {
				return report, fmt.Errorf("rule %s: %w", rule.Name, err)
			}
			in.Rules = append(in.Rules, RuleOutcome{Rule: rule.Name, Result: v})
			report.setVar(vars, rule.RID, "rid", v.ResultID)
			code := v.Describe()
			if v.Result != nil {
				code = ResultName(*v.Result)
			}
			report.setVar(vars, rule.ResVar, "resvar", code)
		}
		report.Intents = append(report.Intents, in)
	}
	return report, nil
}

// Print writes the report as a readable summary.
func (r *AttestReport) Print(pv *Provisioner) {
	w := pv.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Attestation of %s (element %s), session %s, at %s UTC\n",
		pv.P.Element.Name, r.ElementID, r.SessionID, pv.timeNow())
	for _, in := range r.Intents {
		fmt.Fprintf(w, "\n  %s on %s", in.IntentID, in.Endpoint)
		if in.ClaimID != "" {
			fmt.Fprintf(w, "  (claim %s)", in.ClaimID)
		}
		fmt.Fprintln(w)
		if in.Err != nil {
			fmt.Fprintln(w, "    ERROR  attestation failed:", in.Err)
			continue
		}
		if len(in.Rules) == 0 {
			fmt.Fprintln(w, "    -      no rules listed; claim collected only")
		}
		for _, ro := range in.Rules {
			if ro.NotRun != "" {
				fmt.Fprintf(w, "    NOTRUN %s: %s\n", ro.Rule, ro.NotRun)
			} else if ro.Passed() {
				fmt.Fprintf(w, "    PASS   %s\n", ro.Rule)
			} else {
				fmt.Fprintf(w, "    FAIL   %s: %s\n", ro.Rule, ro.Result.Describe())
			}
		}
	}
	if len(r.Vars) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %-20s | %-6s | %s\n", "var", "kind", "value")
		for _, v := range r.Vars {
			fmt.Fprintf(w, "  %-20s | %-6s | %s\n", v.Name, v.Kind, v.Value)
		}
	}
	passed, failed, errs := r.Counts()
	verdict := "PASSED"
	if !r.Trusted() {
		verdict = "FAILED"
	}
	fmt.Fprintf(w, "\nAttestation %s: %d rules passed, %d failed, %d of %d intents could not be attested\n",
		verdict, passed, failed, errs, len(r.Intents))
}
