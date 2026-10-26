package main

import "testing"

// The shipped skeletons (#125) must load: syntax, $variables defined before
// use. That their intent and rule names exist is checked in janeserver
// (rules/skeletons_test.go), which has the lists.
func TestShippedProvisioningFilesLoad(t *testing.T) {
	for _, f := range []string{
		"skeleton/full.yaml",
		"skeleton/full-vm.yaml",
		"skeleton/tpm2-nouefi.yaml",
		"skeleton/sysonly.yaml",
		"testdata/examplecreateprovisioningfile.yaml",
	} {
		p, err := loadProvisioningFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		// every skeleton has the system information (decision 51)
		if len(p.EVS) == 0 || p.EVS[0].IntentID != "std::intent::sys::info" {
			t.Errorf("%s: does not start with std::intent::sys::info", f)
		}
	}
}
