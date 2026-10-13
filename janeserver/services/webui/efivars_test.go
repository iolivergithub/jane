package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"strings"
	"testing"

	"a10/utilities"
)

func efivarsBody(t *testing.T, vars ...map[string]interface{}) map[string]interface{} {
	j, err := json.Marshal(map[string]interface{}{"efivars": vars, "count": len(vars)})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(j, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func raw(name, guid string, attrs int, data []byte) map[string]interface{} {
	return map[string]interface{}{"name": name, "guid": guid, "attributes": attrs, "somevalue": len(data),
		"value": string(data), "errorvalue": "", "rawvalue": base64.StdEncoding.EncodeToString(data)}
}

func TestEfiVarsView(t *testing.T) {
	g := utilities.EfiGlobalVariableGUID
	bootorder := []byte{1, 0, 3, 0}
	boot0001 := append([]byte{1, 0, 0, 0, 4, 0}, []byte{'u', 0, 'b', 0, 'u', 0, 'n', 0, 't', 0, 'u', 0, 0, 0, 0x7f, 0xff, 4, 0}...)
	pk := bytes.Repeat([]byte{0xa1, 0x59, 0xc0}, 40)
	pkHash := sha256.Sum256(pk)

	body := efivarsBody(t,
		raw("BootOrder", g, 7, bootorder),
		raw("Boot0001", g, 7, boot0001),
		raw("PK", g, 0x27, pk),
		map[string]interface{}{"name": "SecureBoot", "guid": g, "attributes": 6, "somevalue": 1, "value": "\x01\x00\x00\x00\x00\x00\x00\x00", "errorvalue": ""},
		map[string]interface{}{"name": "KEK", "guid": g, "attributes": 0x27, "somevalue": 3, "value": string([]byte{0xa1, 0x59, 0xc0}), "errorvalue": ""},
		map[string]interface{}{"name": "Weird", "guid": "12345678-0000-0000-0000-000000000000", "attributes": 0, "somevalue": 0, "value": "\x00\x00\x00\x00", "errorvalue": "buffer too small"},
	)

	v := EfiVarsBody(body)
	got := map[string]EfiVar{}
	for _, e := range v.Vars {
		got[e.Name] = e
	}

	check := func(what, have, want string) {
		t.Helper()
		if have != want {
			t.Errorf("%v: got %q want %q", what, have, want)
		}
	}
	check("BootOrder decoded", got["BootOrder"].Decoded, "Boot0001, Boot0003")
	check("Boot0001 decoded", got["Boot0001"].Decoded, `"ubuntu" (active)`)
	check("SecureBoot decoded", got["SecureBoot"].Decoded, "Enabled (1)")
	check("SecureBoot EV", got["SecureBoot"].EVValue, "01")
	check("SecureBoot rule", got["SecureBoot"].RuleName, "UEFI_SecureBoot")
	check("PK EV", got["PK"].EVValue, "sha256:"+hex.EncodeToString(pkHash[:]))
	check("PK flags", strings.Join(got["PK"].AttributeFlags, ","), "NV,BS,RT,TA")
	check("BootOrder rule", got["BootOrder"].RuleName, "")

	if !got["KEK"].Lossy || !v.AnyLossy || got["KEK"].EVValue != "" {
		t.Errorf("KEK from an older agent should be lossy with no EV value: %+v", got["KEK"])
	}
	if got["Weird"].Error == "" || got["Weird"].HexDump != "" || v.Errors != 1 {
		t.Errorf("Weird: %+v", got["Weird"])
	}
	if v.Vars[len(v.Vars)-1].Name != "Weird" {
		t.Errorf("variables with unknown GUIDs should sort last")
	}
	if v.Count != 6 {
		t.Errorf("count %d", v.Count)
	}

	// and the template renders it
	tm := template.Must(template.New("x").Funcs(templateFunctions()).ParseFS(WPFS, T+"claim_efivars.html"))
	var out bytes.Buffer
	if err := tm.ExecuteTemplate(&out, "claim_efivars.html", body); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{"Boot0001, Boot0003", "approximate", "buffer too small", "0000  01 00 03 00", "Checked by rule UEFI_SecureBoot", "Expected value: <span class=\"font-monospace user-select-all text-break\">01</span>"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered page is missing %q", want)
		}
	}
}
