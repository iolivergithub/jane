package janeapi

import "strings"

// API keys are "jane_" and the unpadded base64url encoding of 32
// random bytes, as addjaneuser --apikey prints them.
const (
	APIKeyPrefix  = "jane_"
	APIKeyEncoded = 43 // base64url of 32 bytes, unpadded
)

// ValidAPIKey reports whether k has the form of a Jane API key. It says
// nothing about whether the key is known to Jane.
func ValidAPIKey(k string) bool {
	if len(k) != len(APIKeyPrefix)+APIKeyEncoded || !strings.HasPrefix(k, APIKeyPrefix) {
		return false
	}
	for _, ch := range k[len(APIKeyPrefix):] {
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}
