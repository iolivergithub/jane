# localnames

Multicast DNS for host names ending in `.local`, for janeserver and jp (#127).
Pure Go ([pion/mdns](https://github.com/pion/mdns)), so the builds stay
without cgo on every architecture.

`localnames.Install()` (called once in each `main`) wraps
`http.DefaultTransport`'s dialer: every name is dialled as before, and only a
DNS failure for a `.local` name is retried with the address from multicast
DNS. Normal DNS, `/etc/hosts` and IP addresses are unchanged.

To remove it, delete the `localnames.Install()` call and the `localnames`
lines in `janeserver/go.mod` and `provisioner/go.mod`, then this directory.

See [docs/running.md](../docs/running.md#host-names-ending-in-local).
