package main

import (
	"bytes"
	"strings"
	"testing"
)

// #144: an error claim (tarzan unreachable, or without -sys) was reported as
// "claim body has no machineid", which looked like a missing /etc/machine-id.
func TestProcessEVsErrorClaimSaysWhy(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.claimType = "*ERROR"
	h.jane.claimBody = `{"error":"dial tcp 192.168.56.1:8530: connect: connection refused"}`
	h.pv.P.EVS = []EVEntry{{IntentID: "std::intent::sys::info", Spec: EVSpec{Protocol: "tarzan", Type: "sysmachineid"}}}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	_, err := h.pv.ProcessWorklist()
	if err == nil {
		t.Fatal("expected an error for an error claim")
	}
	for _, want := range []string{"std::intent::sys::info", "error claim", "connection refused", "-sys"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if n := h.jane.count("POST /expectedValue") + h.jane.count("PUT /expectedValue"); n != 0 {
		t.Errorf("%d expected values written from an error claim", n)
	}
}

// No machine ID is not an error: jp warns and records no expected value.
func TestProcessEVsEmptyMachineID(t *testing.T) {
	h := newHarness(t, "create")
	var out bytes.Buffer
	h.pv.Out = &out
	h.jane.claimBody = `{"machineid":""}`
	h.pv.P.EVS = []EVEntry{{IntentID: "std::intent::sys::info", Spec: EVSpec{Protocol: "tarzan", Type: "sysmachineid"}}}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "found no machine ID") {
		t.Errorf("no warning in output:\n%s", out.String())
	}
	if n := h.jane.count("POST /expectedValue") + h.jane.count("PUT /expectedValue"); n != 0 {
		t.Errorf("%d expected values written without a machine ID", n)
	}
}

// A TPM without an EK certificate gives an error claim; that is still not an
// error for create (nothing is recorded).
func TestProcessEVsErrorClaimEKCertStillTolerated(t *testing.T) {
	h := newHarness(t, "create")
	h.jane.claimType = "*ERROR"
	h.jane.claimBody = `{"error":"No NV index at 0x01c00002"}`
	h.pv.P.EVS = []EVEntry{{IntentID: "std::intent::tpm2::getcert", Spec: EVSpec{Protocol: "tarzan", Type: "tpm2ekcert"}}}
	h.pv.P.ProvisionWorklist = []string{"processelement", "processevs"}
	if _, err := h.pv.ProcessWorklist(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
