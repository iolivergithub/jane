package webui

import (
	"reflect"
	"testing"
)

// Template helpers for the register naming schemes (#101).
func TestPCRTemplateFunctions(t *testing.T) {
	if got := SplitList("0, 1,2 ,"); !reflect.DeepEqual(got, []string{"0", "1", "2"}) {
		t.Errorf("SplitList = %v", got)
	}
	if got := SplitList(nil); len(got) != 0 {
		t.Errorf("SplitList(nil) = %v", got)
	}
	d := Dict("Body", 1, "Scheme", "x", 7)
	if d["Body"] != 1 || d["Scheme"] != "x" || len(d) != 2 {
		t.Errorf("Dict = %v", d)
	}
	if r := PCRName("linux-uefi", 10); r == nil || r.Short != "IMA" {
		t.Errorf("PCRName(linux-uefi, 10) = %+v", r)
	}
	if v := PCRScheme("nonsense"); !v.Unknown || v.Key != "tcg-pcclient-uefi" {
		t.Errorf("PCRScheme(nonsense) = %+v", v)
	}
	if v := PCRScheme(""); v.Unknown {
		t.Errorf("PCRScheme(\"\") flagged unknown")
	}
}
