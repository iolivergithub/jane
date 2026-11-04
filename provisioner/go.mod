module jp

go 1.26.0

require (
	github.com/google/go-eventlog v0.0.2
	github.com/google/go-tpm v0.9.8
	golang.org/x/sys v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

require google.golang.org/protobuf v1.34.2 // indirect

// localnames: multicast DNS for <name>.local (#127), in this repository
require github.com/iolivergithub/jane/lib/localnames v0.0.0

replace github.com/iolivergithub/jane/lib/localnames => ../lib/localnames
