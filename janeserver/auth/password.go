package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Passwords are stored as Argon2id hashes in the standard PHC string format:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
//
// with the salt and hash in unpadded standard base64. This is the format produced by the
// usual Argon2 libraries (e.g. Python's argon2-cffi, used by etc/addjaneuser/addjaneuser), so users can
// be created outside Jane. The parameters are read from each stored hash, so they can be
// raised later without invalidating existing passwords.
//
// Jane only verifies passwords; it never creates or changes them.

// Limits on what a stored hash may ask for, so a corrupted or tampered record cannot make
// a login attempt consume excessive memory or time.
const (
	maxMemoryKiB    = 1 << 20 // 1 GiB
	maxIterations   = 16
	maxParallelism  = 16
	minSaltLen      = 8
	minHashLen      = 16
	maxHashLen      = 128
	maxPasswordSize = 1024 // bytes; longer submissions are rejected before hashing
)

// argon2Params are the parameters, salt and hash decoded from a stored PHC string.
type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
}

// dummyHash is verified against when a username does not exist, so that an unknown
// user takes as long to reject as a wrong password. It uses the recommended parameters
// and a random password nobody knows.
var dummyHash = func() string {
	salt := make([]byte, 16)
	pw := make([]byte, 32)
	_, _ = rand.Read(salt)
	_, _ = rand.Read(pw)
	h := argon2.IDKey(pw, salt, 2, 19456, 1, 32)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=19456,t=2,p=1$%s$%s", argon2.Version, b64.EncodeToString(salt), b64.EncodeToString(h))
}()

// checkPassword reports whether the given password matches the stored Argon2id hash.
// Anything that is not a well-formed Argon2id PHC string (including plain text) never matches.
//
// This is the only place that knows how passwords are stored.
func checkPassword(stored, given string) bool {
	if len(given) > maxPasswordSize {
		return false
	}
	p, err := parseArgon2id(stored)
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(given), p.salt, p.iterations, p.memory, p.parallelism, uint32(len(p.hash)))
	return subtle.ConstantTimeCompare(computed, p.hash) == 1
}

// parseArgon2id decodes "$argon2id$v=19$m=...,t=...,p=...$salt$hash".
func parseArgon2id(s string) (*argon2Params, error) {
	parts := strings.Split(s, "$")
	// parts[0] is empty because the string starts with "$"
	if len(parts) != 6 || parts[0] != "" {
		return nil, fmt.Errorf("not a PHC string")
	}
	if parts[1] != "argon2id" {
		return nil, fmt.Errorf("unsupported algorithm %q", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, fmt.Errorf("unsupported argon2 version %q", parts[2])
	}

	p := &argon2Params{}
	var parallelism uint32
	if n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &parallelism); err != nil || n != 3 {
		return nil, fmt.Errorf("bad parameters %q", parts[3])
	}
	if fmt.Sprintf("m=%d,t=%d,p=%d", p.memory, p.iterations, parallelism) != parts[3] {
		return nil, fmt.Errorf("bad parameters %q", parts[3])
	}
	if p.iterations < 1 || p.iterations > maxIterations ||
		parallelism < 1 || parallelism > maxParallelism ||
		p.memory < 8*parallelism || p.memory > maxMemoryKiB {
		return nil, fmt.Errorf("parameters out of range %q", parts[3])
	}
	p.parallelism = uint8(parallelism)

	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(p.salt) < minSaltLen {
		return nil, fmt.Errorf("bad salt")
	}
	if p.hash, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil || len(p.hash) < minHashLen || len(p.hash) > maxHashLen {
		return nil, fmt.Errorf("bad hash")
	}
	return p, nil
}
