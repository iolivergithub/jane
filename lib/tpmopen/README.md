# tpmopen

Opens a TPM 2.0 from a device string, for jp and tarzan: `""` (the platform
default), a device path such as `/dev/tpmrm0`, `unix:///path` (or a path that
is a socket) for a simulator such as swtpm, or `tcp://host:port` for a
simulator speaking the Microsoft simulator protocol. On Windows TPM Base
Services is used and a path is ignored. See `Open` in `open.go`.

tarzan takes the device from the caller's `tpm2/device` parameter, so it
refuses `tcp://` (`IsNetwork`) unless started with `-unsafe`.

It is the module `github.com/iolivergithub/jane/lib/tpmopen`, used through a
`replace` to `../lib/tpmopen` in `provisioner/go.mod` and `tarzan/go.mod`.
