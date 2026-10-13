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
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- canonicalisation ---

func TestCanonicaliseSortsAndIsStable(t *testing.T) {
	v := map[string]any{"b": 1, "a": []any{true, nil, "x"}, "c": map[string]any{"z": 1.5, "y": "q"}}
	want := `{"a":[true,null,"x"],"b":1,"c":{"y":"q","z":1.5}}`
	for i := 0; i < 50; i++ { // map iteration order is random; output must not be
		got, err := Canonicalise(v)
		if err != nil || string(got) != want {
			t.Fatalf("got %s (%v), want %s", got, err, want)
		}
	}
}

func TestCanonicaliseIntegersExact(t *testing.T) {
	// int64 nanosecond timestamps are > 2^53 and must not be rounded
	got, _ := Canonicalise(map[string]any{"t": int64(1760344800123456789)})
	if string(got) != `{"t":1760344800123456789}` {
		t.Fatalf("got %s", got)
	}
}

func TestCanonicaliseRFC8785Numbers(t *testing.T) {
	// values from RFC 8785 appendix B
	cases := map[float64]string{
		1e21: "1e+21", 1e-7: "1e-7", 0.000001: "0.000001", 333333333.3333333: "333333333.3333333",
		1e23: "1e+23", 4.50: "4.5", 2e-3: "0.002", 0.000033: "0.000033", -1.5: "-1.5",
		5e-324: "5e-324", 1.7976931348623157e308: "1.7976931348623157e+308",
	}
	for f, want := range cases {
		if got := formatES6(f); got != want {
			t.Errorf("formatES6(%v) = %s, want %s", f, got, want)
		}
	}
}

func TestCanonicaliseStrings(t *testing.T) {
	got, _ := Canonicalise("\u20ac$\u000f\nA'B\"\\\\\"/<>&")
	if string(got) != `"€$\u000f\nA'B\"\\\\\"/<>&"` {
		t.Fatalf("got %s", got)
	}
}

func TestCanonicaliseKeyOrderUTF16(t *testing.T) {
	// RFC 8785 section 3.2.3 example: sorted by UTF-16 code units, not UTF-8 bytes
	v := map[string]any{"\u20ac": 1, "\r": 2, "\U0001F600": 3, "1": 4, "\u0080": 5, "\u00f6": 6, "\ufb33": 7}
	got, _ := Canonicalise(v)
	want := "{\"\\r\":2,\"1\":4,\"\u0080\":5,\"\u00f6\":6,\"\u20ac\":1,\"\U0001F600\":3,\"\ufb33\":7}"
	if string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

// --- signing ---

// verify recomputes the signed bytes from a footer and checks the signature.
func verify(t *testing.T, pub crypto.PublicKey, v any, f Footer) {
	t.Helper()
	canonical, _ := Canonicalise(v)
	d := sha256.Sum256(canonical)
	if string(d[:]) != string(f.Hash) {
		t.Fatalf("hash mismatch")
	}
	kid, _ := KeyID(pub)
	if kid != f.Kid || !strings.HasPrefix(kid, "sha256:") {
		t.Fatalf("kid %v, want %v", f.Kid, kid)
	}
	tbs, _ := Canonicalise(map[string]any{"alg": f.Alg, "hash": f.Hash, "kid": f.Kid})
	switch k := pub.(type) {
	case *rsa.PublicKey:
		h := sha256.Sum256(tbs)
		if err := rsa.VerifyPSS(k, crypto.SHA256, h[:], f.Signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}); err != nil {
			t.Fatalf("RSA verify: %v", err)
		}
	case *ecdsa.PublicKey:
		h := sha256.Sum256(tbs)
		r := new(big.Int).SetBytes(f.Signature[:32])
		s := new(big.Int).SetBytes(f.Signature[32:])
		if len(f.Signature) != 64 || !ecdsa.Verify(k, h[:], r, s) {
			t.Fatalf("ECDSA verify failed")
		}
	case ed25519.PublicKey:
		if !ed25519.Verify(k, tbs, f.Signature) {
			t.Fatalf("Ed25519 verify failed")
		}
	}
}

func withKey(t *testing.T, key crypto.Signer, alg string) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "k.pem")
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600)

	s, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	once.Do(func() {})      // consume the once so HashAndSign does not re-initialise
	state, initErr = s, nil // and install the test key

	payload := map[string]any{"body": map[string]any{"pcr": "abc", "n": 3}, "t": int64(1760344800123456789)}
	f, err := HashAndSign(payload)
	if err != nil {
		t.Fatal(err)
	}
	if f.Alg != alg {
		t.Fatalf("alg %v want %v", f.Alg, alg)
	}
	verify(t, key.Public(), payload, f)

	// tampering with the payload must change the hash
	payload["t"] = int64(1760344800123456788)
	f2, _ := HashAndSign(payload)
	if string(f2.Hash) == string(f.Hash) {
		t.Fatalf("hash unchanged after tampering")
	}
}

func TestSignRSA(t *testing.T) {
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	withKey(t, k, "PS256")
}

func TestSignECDSA(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	withKey(t, k, "ES256")
}

func TestSignEd25519(t *testing.T) {
	_, k, _ := ed25519.GenerateKey(rand.Reader)
	withKey(t, k, "EdDSA")
}

func TestKidIsDeterministic(t *testing.T) {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	a, _ := KeyID(k.Public())
	b, _ := KeyID(&k.PublicKey)
	if a != b || len(a) != len("sha256:")+64 {
		t.Fatalf("kid %v / %v", a, b)
	}
}

func TestEphemeral(t *testing.T) {
	s, err := load("")
	if err != nil || s.status.Source != "ephemeral" || s.alg.name != "PS256" {
		t.Fatalf("%v %+v", err, s)
	}
}

func TestBadKeyFile(t *testing.T) {
	if _, err := load("/nonexistent/key.pem"); err == nil {
		t.Fatal("expected error")
	}
}
