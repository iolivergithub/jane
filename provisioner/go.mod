module jp

go 1.26.0

require (
	github.com/google/go-tpm v0.9.8
	golang.org/x/sys v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/google/go-eventlog v0.0.2 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.2.2 // indirect
	golang.org/x/net v0.55.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

// Library modules in this repository (lib/), used through replace so they
// are never downloaded
require (
	github.com/iolivergithub/jane/lib/localnames v0.0.0 // multicast DNS for <name>.local (#127)
	github.com/iolivergithub/jane/lib/uefi v0.0.0 // UEFI variables and measured state, shared with janeserver
)

replace (
	github.com/iolivergithub/jane/lib/localnames => ../lib/localnames
	github.com/iolivergithub/jane/lib/uefi => ../lib/uefi
)
