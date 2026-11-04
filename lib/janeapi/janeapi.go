// Package janeapi holds what jp and Jane must agree on over Jane's REST API:
// the parts of an element jp writes (endpoints, TPM 2.0 keys, host, UEFI and
// IMA sections, attestation profiles), and the formats of profile names and
// API keys. janeserver's structures package and jp use these types through
// type aliases, so the JSON jp sends is by construction what Jane stores.
//
// The struct tags are janeserver's: json for the API, bson for MongoDB, yaml
// for Jane's YAML views and jp's provisioning file.
package janeapi
