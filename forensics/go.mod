module jp_forensics

go 1.26.0

// Library modules in this repository (lib/), used through replace so they
// are never downloaded
require github.com/iolivergithub/jane/lib/janeapi v0.0.0 // API key format, shared with Jane and jp

replace github.com/iolivergithub/jane/lib/janeapi => ../lib/janeapi
