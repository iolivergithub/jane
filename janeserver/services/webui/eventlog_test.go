package webui

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"os"
	"strings"
	"testing"
)

// eventLogClaimBody is a uefi/eventlog claim body as tarzan sends it, for the
// COS 85 test log (see utilities/testdata/README.md).
func eventLogClaimBody(t *testing.T) map[string]interface{} {
	raw, err := os.ReadFile("../../utilities/testdata/cos-85-amd-sev.bin")
	if err != nil {
		t.Fatal(err)
	}
	s := base64.StdEncoding.EncodeToString(raw)
	return map[string]interface{}{"eventlog": s, "encoded": "base64", "unencodedlength": len(raw), "encodedlength": len(s)}
}

func TestEventLogBody(t *testing.T) {
	v := EventLogBody(eventLogClaimBody(t))
	if v.Error != "" {
		t.Fatal(v.Error)
	}
	if v.Count == 0 {
		t.Fatal("no events")
	}
	var names []string
	for _, b := range v.Banks {
		names = append(names, b.Name)
		if len(b.Events) != v.Count {
			t.Errorf("bank %s has %d events, want %d", b.Name, len(b.Events), v.Count)
		}
	}
	if got := strings.Join(names, ","); got != "sha1,sha256,sha384" {
		t.Errorf("banks %s", got)
	}
	if v.DefaultBank != "sha256" {
		t.Errorf("default bank %s", v.DefaultBank)
	}
	if len(v.PCRs) == 0 || v.PCRs[0] != 0 {
		t.Errorf("PCRs %v", v.PCRs)
	}

	var secureBoot, image bool
	for _, e := range v.Banks[1].Events {
		if e.Type == "EV_EFI_VARIABLE_DRIVER_CONFIG" && strings.HasPrefix(e.Description, "SecureBoot (EFI Global Variable)") {
			secureBoot = true
		}
		if e.Type == "EV_EFI_BOOT_SERVICES_APPLICATION" && strings.Contains(e.Description, "bytes") {
			image = true
		}
		if e.Num < 1 {
			t.Errorf("event not numbered: %+v", e)
		}
	}
	if !secureBoot {
		t.Error("SecureBoot variable event not decoded")
	}
	if !image {
		t.Error("no boot application event")
	}

	tm := template.Must(template.New("x").Funcs(templateFunctions()).ParseFS(WPFS, T+"claim_uefieventlog.html", T+"genericList.html"))
	var out bytes.Buffer
	if err := tm.ExecuteTemplate(&out, "claim_uefieventlog.html", eventLogClaimBody(t)); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{`data-bank="sha256"`, `<option value="sha256" selected>`, "EV_EFI_VARIABLE_DRIVER_CONFIG", "SecureBoot"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered page is missing %q", want)
		}
	}
}

func TestEventLogBodyErrors(t *testing.T) {
	for name, body := range map[string]map[string]interface{}{
		"no field":   {},
		"not base64": {"eventlog": "!!!"},
		"not a log":  {"eventlog": base64.StdEncoding.EncodeToString([]byte("not an event log at all"))},
	} {
		if v := EventLogBody(body); v.Error == "" {
			t.Errorf("%s: no error", name)
		}
	}
}
