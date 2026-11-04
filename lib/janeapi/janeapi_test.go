package janeapi

import (
	"strings"
	"testing"
)

func TestValidProfileName(t *testing.T) {
	for _, n := range []string{"default", "nightly", "Host_check-2", strings.Repeat("a", 64)} {
		if !ValidProfileName(n) {
			t.Errorf("%q refused", n)
		}
	}
	for _, n := range []string{"", "a b", "a.b", "a$b", "x/y", strings.Repeat("a", 65)} {
		if ValidProfileName(n) {
			t.Errorf("%q accepted", n)
		}
	}
}

func TestValidAPIKey(t *testing.T) {
	good := APIKeyPrefix + strings.Repeat("A", 20) + "-_09az" + strings.Repeat("Z", 17)
	if !ValidAPIKey(good) {
		t.Errorf("%q refused", good)
	}
	for _, k := range []string{"", good[:len(good)-1], good + "A", "jane-" + good[5:], good[:10] + "+" + good[11:], good[:10] + "=" + good[11:]} {
		if ValidAPIKey(k) {
			t.Errorf("%q accepted", k)
		}
	}
}
