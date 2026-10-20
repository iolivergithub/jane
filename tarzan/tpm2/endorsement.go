package tpm2

import (
	"os"

	"github.com/google/go-tpm/tpm2"
)

// EndorsementPasswordVar names the environment variable holding the TPM's
// endorsement hierarchy password (#76). Most TPMs have none; then the variable
// is unset and the empty password is used, as before. The password stays on
// the element: Jane never sends it, since call parameters are stored in claims.
// On a deb install systemd reads it from /etc/opt/jane/tarzan.env.
const EndorsementPasswordVar = "JANE_TPM_ENDORSEMENT_PASSWORD"

// endorsementPassword is read once, when tarzan starts.
var endorsementPassword = os.Getenv(EndorsementPasswordVar)

// EndorsementPasswordSet reports whether a non-empty password was given.
func EndorsementPasswordSet() bool { return endorsementPassword != "" }

// endorsementAuth authorises use of the endorsement hierarchy in the EK's
// policy session (PolicySecret). TPM2_GetTime cannot use it: see gettime.go.
func endorsementAuth() tpm2.Session {
	return tpm2.PasswordAuth([]byte(endorsementPassword))
}
