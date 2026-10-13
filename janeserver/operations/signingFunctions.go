package operations

import (
	"a10/signing"
	"a10/structures"
)

// footer is any of the three footer structures; each has the same fields as signing.Footer.
type footer interface {
	structures.ClaimFooter | structures.ResultFooter | structures.SessionFooter
}

// hashAndSign is the one generic function behind the three below. All of the hashing,
// canonicalisation, key handling, alg and kid live in the signing package.
func hashAndSign[F footer](h any) (F, error) {
	f, err := signing.HashAndSign(h)
	return F(f), err
}

func hashAndSignClaim(h hashablePartClaim) (structures.ClaimFooter, error) {
	return hashAndSign[structures.ClaimFooter](h)
}

func hashAndSignResult(h hashablePartResult) (structures.ResultFooter, error) {
	return hashAndSign[structures.ResultFooter](h)
}

func hashAndSignSession(h hashablePartSession) (structures.SessionFooter, error) {
	return hashAndSign[structures.SessionFooter](h)
}
