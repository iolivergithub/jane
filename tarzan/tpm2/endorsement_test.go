package tpm2

import "testing"

func TestEndorsementPasswordSet(t *testing.T) {
	defer func(p string) { endorsementPassword = p }(endorsementPassword)
	endorsementPassword = ""
	if EndorsementPasswordSet() {
		t.Error("empty password reported as set")
	}
	endorsementPassword = "s3cret"
	if !EndorsementPasswordSet() {
		t.Error("password not reported as set")
	}
	if endorsementAuth() == nil {
		t.Error("no session")
	}
}
