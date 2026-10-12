package auth

import (
	"strings"
	"testing"
)

// Reference hashes produced by etc/addjaneuser (Python argon2-cffi), so these tests
// also check that users created by the script can log in to Jane.
const (
	pw = "correct horse battery staple"

	// etc/addjaneuser's own parameters
	scriptHash = "$argon2id$v=19$m=19456,t=2,p=1$fbQdSmwVfYYtQMqjwAvS8Q$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k"
	// different parameters, which must be read from the string
	otherParamsHash = "$argon2id$v=19$m=65536,t=3,p=4$Qz0qF6TGQM524edhGkl5TQ$1YSV58EQH9DjEdwpNCg9c4YwGwduSfwxiiPD/LOtYyo"
	// Argon2i, not Argon2id: must be rejected
	argon2iHash = "$argon2i$v=19$m=65536,t=3,p=4$PSkV+hEAE0noLz5ixkgPwQ$40abmRkWa/OaNEW9vxUfrnTPySCh6E5SS18fDZkel48"
	// non-ASCII password
	unicodePw   = "Croeso, Tervetuloa, Welcome ✓"
	unicodeHash = "$argon2id$v=19$m=19456,t=2,p=1$Z/uCLN6kuWDv7XNgodJDjQ$094xMzBg1aK0k6KAIO+L6fuvlWjjRBkIQdAIcKczdRI"
)

func TestScriptHashesVerify(t *testing.T) {
	cases := []struct {
		name, stored, given string
		want                bool
	}{
		{"script params, right password", scriptHash, pw, true},
		{"script params, wrong password", scriptHash, pw + "!", false},
		{"script params, empty password", scriptHash, "", false},
		{"other params, right password", otherParamsHash, pw, true},
		{"other params, wrong password", otherParamsHash, "x", false},
		{"unicode password", unicodeHash, unicodePw, true},
		{"argon2i rejected", argon2iHash, pw, false},
	}
	for _, c := range cases {
		if got := checkPassword(c.stored, c.given); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPlainTextAndMalformedNeverMatch(t *testing.T) {
	for _, stored := range []string{
		"secret", // plain text as stored before hashing
		"",
		"$argon2id$",
		"$argon2id$v=19$m=19456,t=2,p=1$fbQdSmwVfYYtQMqjwAvS8Q",                                                 // no hash
		"$argon2id$v=16$m=19456,t=2,p=1$fbQdSmwVfYYtQMqjwAvS8Q$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k",     // old version
		"$argon2id$v=19$m=19456,t=0,p=1$fbQdSmwVfYYtQMqjwAvS8Q$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k",     // t=0
		"$argon2id$v=19$m=99999999,t=2,p=1$fbQdSmwVfYYtQMqjwAvS8Q$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k",  // memory too large
		"$argon2id$v=19$m=19456,t=2,p=1,x=3$fbQdSmwVfYYtQMqjwAvS8Q$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k", // trailing junk
		"$argon2id$v=19$m=19456,t=2,p=1$!!!$yJPQH24sqdG1/Gy6XsNWC1DQHo+z1XEZUxweuXFEL5k",                        // bad salt
		"$argon2id$v=19$m=19456,t=2,p=1$fbQdSmwVfYYtQMqjwAvS8Q$AAAA",                                            // hash too short
	} {
		if checkPassword(stored, "secret") || checkPassword(stored, pw) {
			t.Errorf("matched malformed stored value %q", stored)
		}
	}
}

func TestOverlongPasswordRejected(t *testing.T) {
	if checkPassword(scriptHash, strings.Repeat("a", maxPasswordSize+1)) {
		t.Fatal("overlong password accepted")
	}
}

func TestDummyHashIsWellFormed(t *testing.T) {
	p, err := parseArgon2id(dummyHash)
	if err != nil {
		t.Fatalf("dummy hash does not parse: %v", err)
	}
	if p.memory != 19456 || p.iterations != 2 || p.parallelism != 1 {
		t.Fatalf("dummy hash should cost the same as a real one, got %+v", p)
	}
	if checkPassword(dummyHash, "") || checkPassword(dummyHash, pw) {
		t.Fatal("dummy hash matched")
	}
}
