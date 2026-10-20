package webui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #92: every script or stylesheet loaded from another site must carry a
// Subresource Integrity hash, so a changed or compromised CDN file is refused
// by the browser.
func TestExternalResourcesHaveIntegrity(t *testing.T) {
	tag := regexp.MustCompile(`(?is)<(script|link)\b[^>]*>`)
	external := regexp.MustCompile(`(?i)\b(src|href)\s*=\s*"(https?:)?//`)
	comment := regexp.MustCompile(`(?s)<!--.*?-->`)

	var files []string
	for _, pattern := range []string{"templates/*.html", "../../auth/templates/*.html"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("no templates found")
	}
	checked := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, el := range tag.FindAllString(comment.ReplaceAllString(string(b), ""), -1) {
			if !external.MatchString(el) {
				continue
			}
			lower := strings.ToLower(el)
			if strings.HasPrefix(lower, "<link") && !strings.Contains(lower, "stylesheet") {
				continue // e.g. preconnect hints load nothing executable
			}
			checked++
			if !strings.Contains(lower, "integrity=\"sha") || !strings.Contains(lower, "crossorigin=") {
				t.Errorf("%s: external resource without integrity/crossorigin: %s", f, el)
			}
		}
	}
	if checked == 0 {
		t.Error("no external resources found; the check is not seeing the templates")
	}
}
