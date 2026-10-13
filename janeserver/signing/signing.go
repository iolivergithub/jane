// Package signing produces the footers placed on claims, results and sessions.
//
// A footer is {kid, alg, hash, signature}:
//
//	hash      = SHA-256( Canonicalise(hashable part) )
//	tbs       = Canonicalise( {"alg": alg, "hash": base64(hash), "kid": kid} )
//	signature = Sign(tbs) with the configured key, using alg
//	kid       = "sha256:" + hex( SHA-256( PKIX DER of the public key ) )
//
// alg and kid are inside the signed bytes so they cannot be swapped without breaking the
// signature. alg uses JOSE names (RFC 7518 / RFC 8037): PS256, ES256, ES384, ES512, EdDSA.
// ECDSA signatures are raw r||s as in JOSE, not ASN.1 DER.
//
// The key can come from anywhere that yields a crypto.Signer. Today: a PEM file named in the
// configuration, or, when none is configured, an ephemeral RSA-2048 key generated at startup
// (signatures made with that key cannot be verified after Jane stops).
package signing

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"sync"
)

// Footer is the generic footer. The ClaimFooter, ResultFooter and SessionFooter structures
// have the same fields in the same order so that a Footer converts directly to each of them.
type Footer struct {
	Kid       string `json:"kid" bson:"kid"`
	Alg       string `json:"alg" bson:"alg"`
	Hash      []byte `json:"hash" bson:"hash"`
	Signature []byte `json:"signature" bson:"signature"`
}

// Status describes the signing key in use, for display (e.g. on the Web UI home page).
type Status struct {
	Source  string // "file" or "ephemeral"
	KeyFile string // path of the key file, if Source is "file"
	KeyType string // e.g. RSA-2048, ECDSA-P-256, Ed25519
	Alg     string
	Kid     string
	Error   string // set if initialisation failed
}

// algorithm describes how a key type signs.
type algorithm struct {
	name    string
	prehash crypto.Hash       // 0: the signer is given the message itself (Ed25519, ML-DSA)
	opts    crypto.SignerOpts // passed to crypto.Signer.Sign
	ecSize  int               // >0: convert ECDSA DER to fixed-size r||s of this many bytes each
}

type signerState struct {
	signer crypto.Signer
	alg    algorithm
	status Status
}

var (
	once    sync.Once
	state   *signerState
	initErr error
)

// Initialise loads the signing key. keyfile is a PEM private key (PKCS#8, PKCS#1 or SEC1);
// if it is empty an ephemeral RSA-2048 key is generated instead.
//
// It should be called once at startup, after the configuration has been read. If it is never
// called, the first HashAndSign initialises with an ephemeral key.
func Initialise(keyfile string) error {
	once.Do(func() {
		state, initErr = load(keyfile)
	})
	return initErr
}

// CurrentStatus reports the key in use.
func CurrentStatus() Status {
	if err := Initialise(""); err != nil {
		return Status{Error: err.Error()}
	}
	return state.status
}

// HashAndSign canonicalises v, hashes it and signs the result. See the package comment.
func HashAndSign(v any) (Footer, error) {
	if err := Initialise(""); err != nil {
		return Footer{}, fmt.Errorf("signing not initialised: %w", err)
	}
	s := state

	canonical, err := Canonicalise(v)
	if err != nil {
		return Footer{}, err
	}
	digest := sha256.Sum256(canonical)

	tbs, err := Canonicalise(struct {
		Alg  string `json:"alg"`
		Hash []byte `json:"hash"`
		Kid  string `json:"kid"`
	}{s.alg.name, digest[:], s.status.Kid})
	if err != nil {
		return Footer{}, err
	}

	sig, err := s.sign(tbs)
	if err != nil {
		return Footer{}, fmt.Errorf("signing failed: %w", err)
	}

	return Footer{Kid: s.status.Kid, Alg: s.alg.name, Hash: digest[:], Signature: sig}, nil
}

func (s *signerState) sign(message []byte) ([]byte, error) {
	input := message
	if s.alg.prehash != 0 {
		h := s.alg.prehash.New()
		h.Write(message)
		input = h.Sum(nil)
	}

	sig, err := s.signer.Sign(rand.Reader, input, s.alg.opts)
	if err != nil {
		return nil, err
	}

	if s.alg.ecSize > 0 {
		return ecdsaDERToRaw(sig, s.alg.ecSize)
	}
	return sig, nil
}

// load obtains a crypto.Signer from the given source and works out alg and kid for it.
// Further sources (PKCS#11, TPM, KMS) can be added here: anything returning a crypto.Signer.
func load(keyfile string) (*signerState, error) {
	var signer crypto.Signer
	var err error
	status := Status{}

	if keyfile == "" {
		fmt.Printf("WARNING: no signing key configured (signing: keyfile:). Generating an ephemeral RSA-2048 key; footers signed with it cannot be verified after Jane stops.\n")
		signer, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generating ephemeral key: %w", err)
		}
		status.Source = "ephemeral"
	} else {
		signer, err = loadPEMFile(keyfile)
		if err != nil {
			return nil, err
		}
		status.Source = "file"
		status.KeyFile = keyfile
	}

	alg, keytype, err := algorithmFor(signer.Public())
	if err != nil {
		return nil, err
	}
	kid, err := KeyID(signer.Public())
	if err != nil {
		return nil, err
	}

	status.KeyType = keytype
	status.Alg = alg.name
	status.Kid = kid

	fmt.Printf("JANE: footer signing with %v key %v (%v), source %v\n", keytype, kid, alg.name, status.Source)
	return &signerState{signer, alg, status}, nil
}

// KeyID is "sha256:" followed by the hex SHA-256 of the PKIX (SubjectPublicKeyInfo) DER
// encoding of the public key. Anyone with the public key can recompute it.
func KeyID(pub crypto.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("kid: cannot encode public key: %w", err)
	}
	h := sha256.Sum256(der)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// algorithmFor is the algorithm table. Adding a key type (e.g. ML-DSA) means adding a case.
func algorithmFor(pub crypto.PublicKey) (algorithm, string, error) {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return algorithm{
			name:    "PS256",
			prehash: crypto.SHA256,
			opts:    &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256},
		}, fmt.Sprintf("RSA-%d", k.N.BitLen()), nil
	case *ecdsa.PublicKey:
		switch k.Curve {
		case elliptic.P256():
			return algorithm{name: "ES256", prehash: crypto.SHA256, opts: crypto.SHA256, ecSize: 32}, "ECDSA-P-256", nil
		case elliptic.P384():
			return algorithm{name: "ES384", prehash: crypto.SHA384, opts: crypto.SHA384, ecSize: 48}, "ECDSA-P-384", nil
		case elliptic.P521():
			return algorithm{name: "ES512", prehash: crypto.SHA512, opts: crypto.SHA512, ecSize: 66}, "ECDSA-P-521", nil
		}
		return algorithm{}, "", fmt.Errorf("unsupported ECDSA curve %v", k.Curve.Params().Name)
	case ed25519.PublicKey:
		return algorithm{name: "EdDSA", prehash: 0, opts: crypto.Hash(0)}, "Ed25519", nil
	}
	return algorithm{}, "", fmt.Errorf("unsupported signing key type %T", pub)
}

// loadPEMFile reads the first private key in a PEM file. Encrypted keys are not supported.
func loadPEMFile(path string) (crypto.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading signing key file %v: %w", path, err)
	}

	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("no private key found in %v", path)
		}

		var key any
		switch block.Type {
		case "PRIVATE KEY":
			key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		case "RSA PRIVATE KEY":
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			key, err = x509.ParseECPrivateKey(block.Bytes)
		case "ENCRYPTED PRIVATE KEY":
			return nil, fmt.Errorf("signing key in %v is encrypted, which is not supported", path)
		default:
			continue // e.g. EC PARAMETERS, CERTIFICATE
		}
		if err != nil {
			return nil, fmt.Errorf("parsing %v in %v: %w", block.Type, path, err)
		}

		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("key in %v (%T) cannot sign", path, key)
		}
		return signer, nil
	}
}

// ecdsaDERToRaw converts an ASN.1 DER ECDSA signature to fixed-size r||s.
func ecdsaDERToRaw(der []byte, size int) ([]byte, error) {
	var rs struct{ R, S *big.Int }
	rest, err := asn1.Unmarshal(der, &rs)
	if err != nil || len(rest) != 0 {
		return nil, fmt.Errorf("malformed ECDSA signature")
	}
	out := make([]byte, 2*size)
	rs.R.FillBytes(out[:size])
	rs.S.FillBytes(out[size:])
	return out, nil
}
