//go:build !windows

package tpm2

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

func TestHandleParam(t *testing.T) {
	cases := []struct {
		in   interface{}
		want tpm2.TPMHandle
		ok   bool
	}{
		{"0x810100AA", 0x810100AA, true},
		{"0X810100ee", 0x810100EE, true},
		{"810100EE", 0x810100EE, true},
		{"", 0, false},
		{"0xZZ", 0, false},
		{"0x1810100EE", 0, false},
		{42, 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		h, err := handleParam(map[string]interface{}{"k": c.in}, "k")
		if (err == nil) != c.ok || h != c.want {
			t.Errorf("handleParam(%v) = %v, %v; want %v, ok %v", c.in, h, err, c.want, c.ok)
		}
	}
}

// Bad or missing parameters are rejected before the TPM is opened, so this
// needs no TPM.
func TestActivateCredentialParameters(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("x"))
	good := map[string]interface{}{
		"tpm2/device":          "/dev/does-not-exist",
		"tpm2/akhandle":        "0x810100AA",
		"tpm2/ekhandle":        "0x810100EE",
		"tpm2/credentialblob":  b64,
		"tpm2/encryptedsecret": b64,
	}
	cases := []struct {
		name, key string
		value     interface{}
		wantInErr string
	}{
		{"no AK handle", "tpm2/akhandle", nil, "tpm2/akhandle"},
		{"bad EK handle", "tpm2/ekhandle", "EK", "tpm2/ekhandle"},
		{"no blob", "tpm2/credentialblob", nil, "tpm2/credentialblob"},
		{"secret not base64", "tpm2/encryptedsecret", "not base64!", "tpm2/encryptedsecret"},
		{"all present, no TPM", "", nil, "Could not open"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := map[string]interface{}{}
			for k, v := range good {
				params[k] = v
			}
			if c.key != "" {
				if c.value == nil {
					delete(params, c.key)
				} else {
					params[c.key] = c.value
				}
			}
			body, _ := json.Marshal(params)
			req := httptest.NewRequest(http.MethodPost, "/tpm2/activatecredential", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			if err := ActivateCredential(echo.New().NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %v, want 422", rec.Code)
			}
			var rtn tpm2taErrorReturn
			if err := json.Unmarshal(rec.Body.Bytes(), &rtn); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rtn.TPM2taError, c.wantInErr) {
				t.Errorf("error %q does not mention %q", rtn.TPM2taError, c.wantInErr)
			}
		})
	}
}

// openTestTPM opens the TPM for TestActivateCredentialTPM: the device in
// TARZAN_TEST_TPM (empty for /dev/tpm0). Builds can replace it, e.g. with an
// in-process simulator.
var openTestTPM = func(t *testing.T) transport.TPMCloser {
	dev, ok := os.LookupEnv("TARZAN_TEST_TPM")
	if !ok {
		t.Skip("set TARZAN_TEST_TPM to a TPM device (e.g. /dev/tpmrm0) to run; it only uses transient objects")
	}
	if dev == "" {
		dev = defaultTPMDevice
	}
	tpm, err := OpenTPM(dev)
	if err != nil {
		t.Fatal(err)
	}
	return tpm
}

// akTemplate is the provisioner's AK: a restricted RSA-2048 RSASSA/SHA-256
// signing key.
var akTemplate = tpm2.TPMTPublic{
	Type:    tpm2.TPMAlgRSA,
	NameAlg: tpm2.TPMAlgSHA256,
	ObjectAttributes: tpm2.TPMAObject{
		FixedTPM:            true,
		FixedParent:         true,
		SensitiveDataOrigin: true,
		UserWithAuth:        true,
		Restricted:          true,
		SignEncrypt:         true,
	},
	Parameters: tpm2.NewTPMUPublicParms(
		tpm2.TPMAlgRSA,
		&tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{Algorithm: tpm2.TPMAlgNull},
			Scheme: tpm2.TPMTRSAScheme{
				Scheme: tpm2.TPMAlgRSASSA,
				Details: tpm2.NewTPMUAsymScheme(
					tpm2.TPMAlgRSASSA,
					&tpm2.TPMSSigSchemeRSASSA{HashAlg: tpm2.TPMAlgSHA256},
				),
			},
			KeyBits: 2048,
		},
	),
	Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{}),
}

// TestActivateCredentialTPM makes a credential the way Jane does for an EK and
// an AK created under it, as the provisioner does, and checks the TPM gives
// the secret back; and that a credential for another key's name is refused.
func TestActivateCredentialTPM(t *testing.T) {
	tpm := openTestTPM(t)
	defer tpm.Close()

	ek, err := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.AuthHandle{Handle: tpm2.TPMRHEndorsement, Auth: tpm2.PasswordAuth(nil)},
		InPublic:      tpm2.New2B(tpm2.RSAEKTemplate),
	}.Execute(tpm)
	if err != nil {
		t.Fatalf("creating EK: %v", err)
	}
	defer tpm2.FlushContext{FlushHandle: ek.ObjectHandle}.Execute(tpm)

	parent := tpm2.AuthHandle{Handle: ek.ObjectHandle, Name: ek.Name, Auth: ekSession()}
	created, err := tpm2.Create{ParentHandle: parent, InPublic: tpm2.New2B(akTemplate)}.Execute(tpm)
	if err != nil {
		t.Fatalf("creating AK: %v", err)
	}
	parent.Auth = ekSession()
	ak, err := tpm2.Load{ParentHandle: parent, InPrivate: created.OutPrivate, InPublic: created.OutPublic}.Execute(tpm)
	if err != nil {
		t.Fatalf("loading AK: %v", err)
	}
	defer tpm2.FlushContext{FlushHandle: ak.ObjectHandle}.Execute(tpm)

	ekPublic, err := ek.OutPublic.Contents()
	if err != nil {
		t.Fatal(err)
	}
	ekKey, err := tpm2.ImportEncapsulationKey(ekPublic)
	if err != nil {
		t.Fatal(err)
	}

	secret := make([]byte, 32)
	rand.Read(secret)
	blob, encSecret, err := tpm2.CreateCredential(rand.Reader, ekKey, ak.Name.Buffer, secret)
	if err != nil {
		t.Fatal(err)
	}

	rtn, err := activateCredential(tpm, ak.ObjectHandle, ek.ObjectHandle, blob, encSecret)
	if err != nil {
		t.Fatalf("activateCredential: %v", err)
	}
	got, _ := base64.StdEncoding.DecodeString(rtn.CertInfo)
	if !bytes.Equal(got, secret) {
		t.Errorf("recovered %x, want %x", got, secret)
	}
	if rtn.AKName == "" || rtn.EKName == "" {
		t.Errorf("names missing: %+v", rtn)
	}

	// A credential made for the EK's name instead of the AK's
	blob, encSecret, err = tpm2.CreateCredential(rand.Reader, ekKey, ek.Name.Buffer, secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := activateCredential(tpm, ak.ObjectHandle, ek.ObjectHandle, blob, encSecret); err == nil {
		t.Error("credential for another name was activated")
	}
}
