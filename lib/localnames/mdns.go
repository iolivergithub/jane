package localnames

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// queryTimeout bounds one multicast DNS lookup.
const queryTimeout = 3 * time.Second

var (
	connOnce sync.Once
	conn     *mdns.Conn
	connErr  error
)

// mdnsConn opens the multicast DNS client once per process: IPv4, and IPv6
// where available. It only asks; it answers for no names. The sockets are
// opened with SO_REUSEADDR (Go's default for multicast), so avahi keeps
// working alongside.
func mdnsConn() (*mdns.Conn, error) {
	connOnce.Do(func() {
		var p4 *ipv4.PacketConn
		var p6 *ipv6.PacketConn
		if a, err := net.ResolveUDPAddr("udp4", mdns.DefaultAddressIPv4); err == nil {
			if l, err := net.ListenUDP("udp4", a); err == nil {
				p4 = ipv4.NewPacketConn(l)
			}
		}
		if a, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6); err == nil {
			if l, err := net.ListenUDP("udp6", a); err == nil {
				p6 = ipv6.NewPacketConn(l)
			}
		}
		if p4 == nil && p6 == nil {
			connErr = fmt.Errorf("cannot open the multicast DNS port 5353")
			return
		}
		conn, connErr = mdns.NewServer(p4, p6)
	})
	return conn, connErr
}

// Resolve asks for the address of name (e.g. central.local) by multicast DNS.
func Resolve(ctx context.Context, name string) (netip.Addr, error) {
	c, err := mdnsConn()
	if err != nil {
		return netip.Addr{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, addr, err := c.QueryAddr(ctx, name)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("no answer for %s: %v", name, err)
	}
	return addr, nil
}
