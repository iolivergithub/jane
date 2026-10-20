package main

import "testing"

// The shipped examples must load: syntax, $variables defined before use.
func TestShippedProvisioningFilesLoad(t *testing.T) {
	for _, f := range []string{
		"skeleton/examplecreateprovisioningfile.yaml",
		"skeleton/examplecreatetestvm.yaml",
		"skeleton/pi.yaml",
		"skeleton/pi_notpm.yaml",
		"../etc/debbuild/provisioning.yaml.example",
	} {
		if _, err := loadProvisioningFile(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
