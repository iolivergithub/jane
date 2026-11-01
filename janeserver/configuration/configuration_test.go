package configuration

import (
	"testing"
	"time"
)

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

// #98: sessions: longsession, in seconds; missing or 0 means 60, under 30 means 30.
func TestLongSession(t *testing.T) {
	old := ConfigData
	defer func() { ConfigData = old }()

	ConfigData = nil
	if got := LongSession(); got != 60*time.Second {
		t.Errorf("no configuration: %v", got)
	}
	for set, want := range map[int]time.Duration{0: 60 * time.Second, 10: 30 * time.Second, -5: 30 * time.Second, 30: 30 * time.Second, 300: 300 * time.Second} {
		ConfigData = &ConfigurationStruct{}
		ConfigData.Sessions.LongSession = set
		if got := LongSession(); got != want {
			t.Errorf("longsession %d: got %v, want %v", set, got, want)
		}
	}
}

// #137: sessions: refresh, in seconds; missing means 2, 0 switches it off.
func TestSessionRefresh(t *testing.T) {
	saved := ConfigData
	defer func() { ConfigData = saved }()
	ConfigData = &ConfigurationStruct{}
	if SessionRefresh() != 2 {
		t.Errorf("missing: %d", SessionRefresh())
	}
	for _, c := range []struct{ set, want int }{{0, 0}, {5, 5}, {-1, 0}} {
		n := c.set
		ConfigData.Sessions.Refresh = &n
		if got := SessionRefresh(); got != c.want {
			t.Errorf("refresh %d: got %d, want %d", c.set, got, c.want)
		}
	}
}
