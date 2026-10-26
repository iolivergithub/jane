package webui

import (
	"reflect"
	"strings"
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

// PCRs are listed 0, 1, 2, ... 10, not in textual order (0, 1, 10, 11, 2, ...).
func TestSortRegisters(t *testing.T) {
	bank := map[string]interface{}{"10": "a", "2": "b", "0": "c", "23": "d", "1": "e", "x": "f"}
	var got []string
	for _, r := range SortRegisters(bank) {
		got = append(got, r.Register+"="+r.Value)
	}
	want := "0=c 1=e 2=b 10=a 23=d x=f"
	if g := strings.Join(got, " "); g != want {
		t.Errorf("got %q, want %q", g, want)
	}
	if len(SortRegisters("not a map")) != 0 {
		t.Error("non-map gave registers")
	}
}
