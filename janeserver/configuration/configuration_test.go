package configuration

import "testing"

// listenon is honoured; an empty value still means all interfaces.
func TestListenAddress(t *testing.T) {
	cases := []struct{ on, port, want string }{
		{"", "8520", ":8520"},
		{"0.0.0.0", "8540", "0.0.0.0:8540"},
		{"127.0.0.1", "3270", "127.0.0.1:3270"},
		{"::1", "8520", "[::1]:8520"},
	}
	for _, c := range cases {
		if got := ListenAddress(c.on, c.port); got != c.want {
			t.Errorf("ListenAddress(%q, %q) = %q, want %q", c.on, c.port, got, c.want)
		}
	}
}
