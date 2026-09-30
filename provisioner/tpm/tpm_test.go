package tpm

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"runtime"
	"slices"
	"testing"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

func TestParseHandle(t *testing.T) {
	for in, want := range map[string]tpm2.TPMHandle{
		"0x810100EE":  0x810100EE,
		"0x810100aa":  0x810100AA,
		" 0x81000001": 0x81000001,
		"2164326638":  0x810100EE, // decimal, as PyYAML turned unquoted hex into
	} {
		got, err := ParseHandle(in)
		if err != nil || got != want {
			t.Errorf("ParseHandle(%q) = %s, %v", in, Hex(got), err)
		}
	}
	for _, bad := range []string{"", "EE", "0x01", "0x80000001", "0x1C000002", "0x1FFFFFFFF"} {
		if _, err := ParseHandle(bad); err == nil {
			t.Errorf("ParseHandle(%q) succeeded", bad)
		}
	}
}

func TestAKTemplate(t *testing.T) {
	a := AKTemplate.ObjectAttributes
	if !a.Restricted || !a.SignEncrypt || a.Decrypt || !a.FixedTPM || !a.FixedParent ||
		!a.SensitiveDataOrigin || !a.UserWithAuth || AKTemplate.NameAlg != tpm2.TPMAlgSHA256 {
		t.Errorf("AK attributes = %+v", a)
	}
	rsaParms, err := AKTemplate.Parameters.RSADetail()
	if err != nil {
		t.Fatal(err)
	}
	scheme, err := rsaParms.Scheme.Details.RSASSA()
	if err != nil || rsaParms.KeyBits != 2048 || scheme.HashAlg != tpm2.TPMAlgSHA256 {
		t.Errorf("AK RSA params = %+v, %v", rsaParms, err)
	}
}

func TestOpenErrors(t *testing.T) {
	bad := []string{"tcp://no-port", "tcp://localhost:notaport", "tcp://localhost:0"}
	if runtime.GOOS != "windows" {
		bad = append(bad, "http://x", "/nonexistent/tpm", "unix:///nonexistent/sock")
	}
	for _, d := range bad {
		if tp, _, err := Open(d); err == nil {
			tp.Close()
			t.Errorf("Open(%q) succeeded", d)
		}
	}
}

// openTestTPM opens the TPM for the integration test. By default it uses the
// device in JP_TEST_TPM (any form Open accepts); builds can replace it, e.g.
// with an in-process simulator.
var openTestTPM = func(t *testing.T) transport.TPMCloser {
	dev, ok := os.LookupEnv("JP_TEST_TPM")
	if !ok {
		t.Skip("set JP_TEST_TPM to a TPM (device path, unix://, tcp://, or empty for the default) to run; " +
			"it creates and evicts keys at 0x817FFFE0 and 0x817FFFE1")
	}
	tp, desc, err := Open(dev)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("using", desc)
	return tp
}

// TestProvisioning runs the full provisioning cycle on a TPM and checks the
// AK really is an attestation key for the EK's TPM by verifying a quote.
func TestProvisioning(t *testing.T) {
	tp := openTestTPM(t)
	defer tp.Close()
	d := &TPM{T: tp}
	const ekH, akH tpm2.TPMHandle = 0x817FFFE0, 0x817FFFE1

	for _, h := range []tpm2.TPMHandle{ekH, akH} {
		if _, err := d.Evict(h); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { d.Evict(akH); d.Evict(ekH) })

	if k, err := d.ReadKey(ekH); k != nil || err != nil {
		t.Fatalf("ReadKey on empty handle = %v, %v", k, err)
	}

	ek, err := d.CreateEK(ekH)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateEK(ekH); !errors.Is(err, ErrHandleInUse) {
		t.Errorf("second CreateEK: %v, want ErrHandleInUse", err)
	}
	// The EK is the TCG default one: recreating from the template in the
	// endorsement hierarchy yields the same key.
	again, err := tpm2.CreatePrimary{PrimaryHandle: endorsementAuth(), InPublic: tpm2.New2B(tpm2.RSAEKTemplate)}.Execute(tp)
	if err != nil {
		t.Fatal(err)
	}
	d.flush(again.ObjectHandle)
	if again.Name.Buffer == nil || string(again.Name.Buffer) != string(ek.Name.Buffer) {
		t.Error("EK differs from the TCG template's primary key")
	}

	ak, err := d.CreateAK(ekH, akH)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateAK(ekH, akH); !errors.Is(err, ErrHandleInUse) {
		t.Errorf("second CreateAK: %v, want ErrHandleInUse", err)
	}
	read, err := d.ReadKey(akH)
	if err != nil || read == nil || string(read.Name.Buffer) != string(ak.Name.Buffer) {
		t.Fatalf("ReadKey(AK) = %v, %v", read, err)
	}
	akPub, err := ak.Public.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := tpm2.ObjectName(akPub); string(name.Buffer) != string(ak.Name.Buffer) {
		t.Error("AK name does not match its public area")
	}

	// janeserver's decoding of the element's ak.public
	raw, err := base64.StdEncoding.DecodeString(read.PublicBase64())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpm2.Unmarshal[tpm2.TPM2BPublic](raw); err != nil {
		t.Fatal(err)
	}

	pemBytes, err := ak.PublicPEM()
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(pemBytes)
	key, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	// Quote PCR 0 with the persisted AK and verify it with the PEM key.
	nonce := []byte("jane provisioner test nonce")
	q, err := tpm2.Quote{
		SignHandle:     tpm2.AuthHandle{Handle: akH, Name: ak.Name, Auth: tpm2.PasswordAuth(nil)},
		QualifyingData: tpm2.TPM2BData{Buffer: nonce},
		InScheme:       tpm2.TPMTSigScheme{Scheme: tpm2.TPMAlgNull},
		PCRSelect: tpm2.TPMLPCRSelection{PCRSelections: []tpm2.TPMSPCRSelection{{
			Hash: tpm2.TPMAlgSHA256, PCRSelect: tpm2.PCClientCompatible.PCRs(0),
		}}},
	}.Execute(tp)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := q.Signature.Signature.RSASSA()
	if err != nil {
		t.Fatal(err)
	}
	attested, err := q.Quoted.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if string(attested.ExtraData.Buffer) != string(nonce) {
		t.Error("quote nonce mismatch")
	}
	digest := sha256.Sum256(q.Quoted.Bytes())
	if err := rsa.VerifyPKCS1v15(key.(*rsa.PublicKey), crypto.SHA256, digest[:], sig.Sig.Buffer); err != nil {
		t.Errorf("quote signature does not verify with ak.pub: %v", err)
	}

	hs, err := d.PersistentHandles()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(hs, ekH) || !slices.Contains(hs, akH) {
		t.Errorf("persistent handles %v missing EK or AK", hs)
	}

	for _, h := range []tpm2.TPMHandle{akH, ekH} {
		if ok, err := d.Evict(h); !ok || err != nil {
			t.Errorf("Evict(%s) = %v, %v", Hex(h), ok, err)
		}
		if ok, err := d.Evict(h); ok || err != nil {
			t.Errorf("second Evict(%s) = %v, %v", Hex(h), ok, err)
		}
	}
	hs, _ = d.PersistentHandles()
	if slices.Contains(hs, ekH) || slices.Contains(hs, akH) {
		t.Errorf("handles still persistent after evict: %v", hs)
	}
}
