package webui

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strings"
	"testing"
)

// Every page template parses with its partials (template.Must panics otherwise).
func TestAllTemplatesParse(t *testing.T) {
	if len(parseTemplates(templateFunctions())) == 0 {
		t.Fatal("no templates")
	}
}

func renderPartial(t *testing.T, name, body string) string {
	t.Helper()
	var b map[string]interface{}
	if err := json.Unmarshal([]byte(body), &b); err != nil {
		t.Fatal(err)
	}
	tm := template.Must(template.New("x").Funcs(templateFunctions()).ParseFS(WPFS, T+name, T+"genericList.html"))
	var out bytes.Buffer
	if err := tm.ExecuteTemplate(&out, name, b); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestQuoteSafeBadge(t *testing.T) {
	for safe, want := range map[string]string{"true": "Safe (true)", "false": "Unsafe (false)"} {
		html := renderPartial(t, "claim_quote.html", `{"quote":{"clockinfo":{"safe":"`+safe+`"},"attested":{}},"signature":{}}`)
		if !strings.Contains(html, want) {
			t.Errorf("safe=%s: page lacks %q", safe, want)
		}
	}
}

func TestGetTimeTemplate(t *testing.T) {
	html := renderPartial(t, "claim_gettime.html", `{"time":{"magic":"ff544347","type":"8019","clock":"x",
		"clockinfo":{"clock":"123","resetcount":"7","restartcount":"0","safe":"false"},"time":"456","firmwareVersion":"1a"},
		"attest":"QUJD","rawsignature":"REVG","signature":{"sigAlg":20}}`)
	for _, want := range []string{"456 ms", "Unsafe (false)", "8019", "QUJD", "REVG"} {
		if !strings.Contains(html, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}

func TestCapabilitiesTemplate(t *testing.T) {
	html := renderPartial(t, "claim_tpm2capabilities.html", `{"summary":{"manufacturer":"IFX","vendorstring":"SLB9670","firmwareversion":"0007003d00000000",
		"family":"2.0","level":"0","revision":"1.38","fips140_2":true},"properties":{"TPM_PT_MANUFACTURER":"IFX"},
		"pcrbanks":{"sha256":24},"algorithms":["TPM_ALG_SHA256"],"ecccurves":[],"persistenthandles":["0x81010001"]}`)
	for _, want := range []string{"IFX", "SLB9670", "revision 1.38", "24 PCRs allocated", "TPM_ALG_SHA256", "0x81010001", "TPM_PT_MANUFACTURER"} {
		if !strings.Contains(html, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(html, "Commands (") {
		t.Errorf("Commands section shown for a claim without commands (tarzan before 1.5.5)")
	}

	html = renderPartial(t, "claim_tpm2capabilities.html", `{"summary":{},"properties":{},"pcrbanks":{},
		"commands":["TPM_CC_Quote (0x158)","vendor command (0x20000002)"]}`)
	for _, want := range []string{"Commands (2 implemented)", "TPM_CC_Quote (0x158)", "vendor command (0x20000002)"} {
		if !strings.Contains(html, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}
