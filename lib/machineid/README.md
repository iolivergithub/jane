# machineid

The host's machine ID, read the same way by jp (which records it on the
element as `host.machineid`) and tarzan (which reports it in `sys/info`
claims), so `sys_machineID_CrossRef` compares like with like. See the package
comment in `machineid.go` for where it is read on each platform.

Up to 1.6 tarzan read only `/etc/machine-id` on Unix while jp also tried
`/var/lib/dbus/machine-id`, `/etc/hostid` and, on macOS and the BSDs, the
kernel's host UUID; on hosts where those differ the rule failed although
nothing had changed.

It is the module `github.com/iolivergithub/jane/lib/machineid`, used through
a `replace` to `../lib/machineid` in `provisioner/go.mod` and `tarzan/go.mod`.
