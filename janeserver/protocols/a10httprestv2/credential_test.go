package a10httprestv2

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"a10/structures"

	"github.com/google/go-tpm/tpm2"
)

// testPublic fills tmpl with a fresh RSA key and returns it as base64
// TPM2B_PUBLIC, the way the provisioner stores keys in an element.
func testPublic(t *testing.T, tmpl tpm2.TPMTPublic) string {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl.Unique = tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{Buffer: k.N.Bytes()})
	return base64.StdEncoding.EncodeToString(tpm2.Marshal(tpm2.New2B(tmpl)))
}

// taStub records the parameters posted to it and answers like tarzan.
func taStub(t *testing.T, posted *map[string]interface{}, calls *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if r.URL.Path != "/tpm2/activatecredential" {
			t.Errorf("posted to %v", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, posted)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"certinfo":"AAAA","akname":"00","ekname":"00"}`))
	}))
}

func credentialElement(t *testing.T) structures.Element {
	var e structures.Element
	e.TPM2.Device = "/dev/tpmrm0"
	e.TPM2.EK = structures.TPMKey{Handle: "0x810100EE", Public: testPublic(t, tpm2.RSAEKTemplate)}
	e.TPM2.AK = structures.TPMKey{Handle: "0x810100AA", Public: testPublic(t, tpm2.RSASRKTemplate)}
	return e
}

// The element supplies the keys, an additional parameter overrides the AK
// handle, and what is sent carries the credential and only the secret's hash.
func TestActivateCredentialParameters(t *testing.T) {
	var posted map[string]interface{}
	calls := 0
	ts := taStub(t, &posted, &calls)
	defer ts.Close()

	e := credentialElement(t)
	ep := structures.Endpoint{Endpoint: ts.URL, Protocol: "A10HTTPRESTv2"}
	p := structures.Intent{ItemID: "std::intent::tpm2::activatecredential", Function: "tpm2/activatecredential", Parameters: map[string]interface{}{}}
	aps := map[string]interface{}{"tpm2/akhandle": "0x81010099"}

	body, cps, bt := Call(e, ep, p, structures.Session{}, aps)
	if bt != "tpm2/activatecredential" {
		t.Fatalf("body type %v, body %v", bt, body)
	}

	want := map[string]interface{}{
		"tpm2/device":   "/dev/tpmrm0",
		"tpm2/akhandle": "0x81010099",
		"tpm2/ekhandle": "0x810100EE",
		"tpm2/akpublic": e.TPM2.AK.Public,
		"tpm2/ekpublic": e.TPM2.EK.Public,
	}
	for k, v := range want {
		if posted[k] != v {
			t.Errorf("posted %v = %v, want %v", k, posted[k], v)
		}
	}
	for _, k := range []string{"tpm2/credentialblob", "tpm2/encryptedsecret", "tpm2/credentialsecrethash"} {
		if s, _ := posted[k].(string); s == "" {
			t.Errorf("posted %v missing", k)
		}
		if cps[k] != posted[k] {
			t.Errorf("claim call parameter %v differs from what was sent", k)
		}
	}
	if len(posted) != len(want)+3 {
		t.Errorf("posted %d parameters, want %d: %v", len(posted), len(want)+3, posted)
	}
}

// An intent parameter replaces the element's key, so the credential is made
// for that key.
func TestActivateCredentialIntentOverride(t *testing.T) {
	var posted map[string]interface{}
	calls := 0
	ts := taStub(t, &posted, &calls)
	defer ts.Close()

	other := testPublic(t, tpm2.RSASRKTemplate)
	p := structures.Intent{Function: "tpm2/activatecredential", Parameters: map[string]interface{}{"tpm2/akpublic": other}}
	Call(credentialElement(t), structures.Endpoint{Endpoint: ts.URL}, p, structures.Session{}, nil)
	if posted["tpm2/akpublic"] != other {
		t.Error("intent tpm2/akpublic did not replace the element's")
	}
}

// Without the keys nothing is sent and the claim is an error.
func TestActivateCredentialNoKeys(t *testing.T) {
	var posted map[string]interface{}
	calls := 0
	ts := taStub(t, &posted, &calls)
	defer ts.Close()

	var e structures.Element
	e.TPM2.AK.Handle, e.TPM2.EK.Handle = "0x810100AA", "0x810100EE"
	body, _, bt := Call(e, structures.Endpoint{Endpoint: ts.URL}, structures.Intent{Function: "tpm2/activatecredential"}, structures.Session{}, nil)
	if bt != structures.CLAIMERROR || calls != 0 {
		t.Errorf("body type %v, %d calls to the TA", bt, calls)
	}
	if body["error"] == nil {
		t.Error("no error in body")
	}
}
