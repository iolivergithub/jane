package localnames

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

func TestIsLocal(t *testing.T) {
	for h, want := range map[string]bool{
		"central.local": true, "Central.LOCAL.": true, "pi.local": true,
		"local": false, ".local": false, "central.local.example.org": false,
		"example.org": false, "192.168.1.8": false, "": false,
	} {
		if IsLocal(h) != want {
			t.Errorf("IsLocal(%q) = %v", h, !want)
		}
	}
}

// fakeDial fails names it does not know with a DNS error, as the resolver does.
func fakeDial(known map[string]bool, dialled *[]string) DialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		*dialled = append(*dialled, address)
		host, _, _ := net.SplitHostPort(address)
		if known[host] {
			c1, c2 := net.Pipe()
			c2.Close()
			return c1, nil
		}
		return nil, &net.OpError{Op: "dial", Net: network, Err: &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}}
	}
}

func TestWrap(t *testing.T) {
	resolved := 0
	resolve := func(ctx context.Context, name string) (netip.Addr, error) {
		resolved++
		if name == "central.local" {
			return netip.MustParseAddr("192.168.1.8"), nil
		}
		return netip.Addr{}, errors.New("timeout")
	}
	known := map[string]bool{"jane.example.org": true, "192.168.1.8": true, "listed.local": true}

	cases := []struct {
		address  string
		ok       bool
		dialled  string
		resolved int
	}{
		{"jane.example.org:8520", true, "jane.example.org:8520", 0}, // normal DNS: untouched
		{"192.168.1.8:8520", true, "192.168.1.8:8520", 0},           // IP address
		{"listed.local:8520", true, "listed.local:8520", 0},         // .local in /etc/hosts: untouched
		{"central.local:8520", true, "central.local:8520 192.168.1.8:8520", 1},
		{"other.local:8520", false, "other.local:8520", 1},             // no mDNS answer
		{"nowhere.example.org:80", false, "nowhere.example.org:80", 0}, // DNS failure elsewhere: not mDNS
	}
	for _, c := range cases {
		var dialled []string
		resolved = 0
		conn, err := Wrap(fakeDial(known, &dialled), resolve)(context.Background(), "tcp", c.address)
		if (err == nil) != c.ok {
			t.Errorf("%s: err %v", c.address, err)
		}
		if conn != nil {
			conn.Close()
		}
		if got := strings.Join(dialled, " "); got != c.dialled || resolved != c.resolved {
			t.Errorf("%s: dialled %q, resolved %d", c.address, got, resolved)
		}
		if c.address == "other.local:8520" && (err == nil || !strings.Contains(err.Error(), "multicast DNS: timeout")) {
			t.Errorf("error does not mention multicast DNS: %v", err)
		}
	}

	// a failure that is not a DNS error (connection refused) is not retried
	refused := func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	}
	resolved = 0
	if _, err := Wrap(refused, resolve)(context.Background(), "tcp", "central.local:8520"); err == nil || resolved != 0 {
		t.Errorf("refused: err %v, resolved %d", err, resolved)
	}
}
