module jp

go 1.26.0

require (
	github.com/google/go-tpm v0.9.8
	golang.org/x/sys v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

// localnames: multicast DNS for <name>.local (#127), in this repository
require localnames v0.0.0

replace localnames => ../localnames
