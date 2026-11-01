// Package localnames lets Jane and jp reach hosts named <name>.local (#127).
//
// The binaries are built without cgo, so they use Go's own resolver, which
// reads /etc/hosts and asks the DNS servers but does not do multicast DNS;
// avahi's nss-mdns, which resolves .local names for programs using glibc, is
// not consulted. Install makes HTTP requests made through
// http.DefaultTransport resolve such names with multicast DNS, in pure Go, when
// the normal resolver cannot:
//
//   - every name is first dialled as before, so DNS, /etc/hosts and IP
//     addresses behave exactly as without this package;
//   - only if that fails with a DNS error, and the name ends in .local, is the
//     address asked for by multicast DNS and dialled.
//
// To remove this (Go's resolver learns mDNS, or another solution), delete the
// localnames.Install() call in each main package and the localnames lines in
// their go.mod files; nothing else refers to this package.
package localnames

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DialFunc is the signature of http.Transport.DialContext.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ResolveFunc returns the address of a .local name.
type ResolveFunc func(ctx context.Context, name string) (netip.Addr, error)

// Install wraps http.DefaultTransport's dialer with Wrap and Resolve. Every
// http.Client without its own Transport uses it.
func Install() {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok || t.DialContext == nil {
		return
	}
	t.DialContext = Wrap(t.DialContext, Resolve)
}

// IsLocal reports whether host is a multicast DNS name (ends in .local).
func IsLocal(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	return strings.HasSuffix(h, ".local") && len(h) > len(".local")
}

// Wrap returns a dial function that tries dial first and, only when that
// fails with a DNS error for a .local host, resolves the host with resolve and
// dials its address instead.
func Wrap(dial DialFunc, resolve ResolveFunc) DialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err == nil {
			return conn, nil
		}
		host, port, splitErr := net.SplitHostPort(address)
		var dnsErr *net.DNSError
		if splitErr != nil || !IsLocal(host) || !errors.As(err, &dnsErr) {
			return nil, err
		}
		addr, rerr := resolve(ctx, strings.TrimSuffix(host, "."))
		if rerr != nil {
			return nil, fmt.Errorf("%w; multicast DNS: %v", err, rerr)
		}
		return dial(ctx, network, net.JoinHostPort(addr.Unmap().String(), port))
	}
}
