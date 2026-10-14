package a10httprestv2

import (
	"fmt"

	"a10/utilities"
)

// addCredential makes the credential for tpm2/activatecredential (#49) from
// the AK and EK public keys in the call parameters, i.e. the element's unless
// the intent or the caller overrode them, and adds it to the call parameters.
// Only the SHA-256 of the secret is added: everything in the call parameters
// is sent to the trust agent, and a secret sent with its own credential would
// prove nothing.
func addCredential(cps map[string]interface{}) error {
	ak, _ := cps["tpm2/akpublic"].(string)
	ek, _ := cps["tpm2/ekpublic"].(string)
	if ak == "" || ek == "" {
		return fmt.Errorf("no AK or EK public key: the element's tpm2.ak.public and tpm2.ek.public are empty (run jp update) and tpm2/akpublic, tpm2/ekpublic were not given")
	}

	c, err := utilities.MakeCredential(ak, ek)
	if err != nil {
		return fmt.Errorf("making credential: %w", err)
	}

	cps["tpm2/credentialblob"] = c.Blob
	cps["tpm2/encryptedsecret"] = c.EncryptedSecret
	cps["tpm2/credentialsecrethash"] = c.SecretHash
	return nil
}
