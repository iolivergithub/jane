package tpm2

import (
	"github.com/google/go-tpm/tpm2"
	_ "github.com/google/go-tpm/tpm2/transport"
	_ "github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

type tpm2taErrorReturn struct {
	TPM2taError string `json:"tpm2taerror"`
}

var npcrbanks = []tpm2.TPMIAlgHash{tpm2.TPMAlgSHA1, tpm2.TPMAlgSHA256, tpm2.TPMAlgSHA384, tpm2.TPMAlgSHA512}

type pcrValue map[int]string

var bankNames = map[tpm2.TPMAlgID]string{
	tpm2.TPMAlgSHA1:   "sha1",
	tpm2.TPMAlgSHA256: "sha256",
	tpm2.TPMAlgSHA384: "sha384",
	tpm2.TPMAlgSHA512: "sha512",
}

var bankValues = map[string]tpm2.TPMAlgID{
	"sha1":   tpm2.TPMAlgSHA1,
	"sha256": tpm2.TPMAlgSHA256,
	"sha384": tpm2.TPMAlgSHA384,
	"sha512": tpm2.TPMAlgSHA512,
}
