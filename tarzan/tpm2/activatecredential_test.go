//go:build !windows

package tpm2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
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
