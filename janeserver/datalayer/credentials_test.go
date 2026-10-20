package datalayer

import "testing"

func TestEnvCredential(t *testing.T) {
	defer func(g func(string) string) { getenv = g }(getenv)
	cases := []struct {
		env        map[string]string
		user, pass string
		ok         bool
	}{
		{map[string]string{}, "", "", false},
		{map[string]string{"U": ""}, "", "", false},
		{map[string]string{"U": "", "P": "secret"}, "", "", false},
		{map[string]string{"U": "jane"}, "jane", "", true},
		{map[string]string{"U": "jane", "P": "secret"}, "jane", "secret", true},
	}
	for _, c := range cases {
		env := c.env
		getenv = func(k string) string { return env[k] }
		u, p, ok := envCredential("U", "P")
		if u != c.user || p != c.pass || ok != c.ok {
			t.Errorf("env %v: got (%q, %q, %v), want (%q, %q, %v)", env, u, p, ok, c.user, c.pass, c.ok)
		}
	}
}
