package webui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"encoding/base64"
	"encoding/hex"

	"html/template"

	"a10/operations"
	"a10/registernames"
	"a10/structures"
	"a10/utilities"

	"github.com/google/go-tpm/legacy/tpm2"
)

func EpochToUTCdetailed(epoch structures.Timestamp) string {
	return epochToUTCformat(epoch, true)
}

func EpochToUTCsimple(epoch structures.Timestamp) string {
	return epochToUTCformat(epoch, false)
}

func epochToUTCformat(epoch structures.Timestamp, detailed bool) string {
	// if detailed = true then format goes down to microseconds
	var tfmt = "2006-01-02 15:04:05"
	if detailed == true {
		tfmt = "2006-01-02 15:04:05.0000000"
	}
	sec, err := strconv.ParseInt(fmt.Sprintf("%v", epoch), 10, 64)
	if err != nil {
		t := time.Unix(0, 0)
		return fmt.Sprintf("%v", t.UTC().Format(tfmt))
	}
	t := time.Unix(0, sec)
	return fmt.Sprintf("%v", t.UTC().Format(tfmt))
}

func DefaultMessage() string {
	return "Invocation from Jane WebUI initiated " + EpochToUTCdetailed(utilities.MakeTimestamp())
}

func Base64decode(u string) string {
	d, _ := base64.StdEncoding.DecodeString(u)
	return string(d)
}

func EncodeAsHexString(b []byte) string {
	return hex.EncodeToString(b)
}

func TCGAlg(h int32) string {
	return tpm2.Algorithm(h).String()
}

func GetOpaqueObjectByValue(v string) template.HTML {
	o, err := operations.GetOpaqueObjectByValue(v)
	if err != nil {
		return template.HTML(v)
	} else {
		sd := o.Type + " : " + o.ShortDescription
		s := `<span data-bs-toggle="tooltip" title="` + sd + `"><a href="/opaqueobject/` + v + `">` + v + `</a></span>`
		return template.HTML(s)
	}
}

func GetOpaqueObjectByValueInt64(v int64) template.HTML {
	s := strconv.FormatInt(v, 10)
	return GetOpaqueObjectByValue(s)
}

// ResultClass groups a result value as "pass", "fail" or "other"; pages use it
// to colour each individual result (it never summarises several results).
func ResultClass(v structures.ResultValue) string {
	return resultOutcome(v)
}

// ResultLabel is the readable name of a result value.
func ResultLabel(v structures.ResultValue) string {
	switch v {
	case structures.Success:
		return "Pass"
	case structures.Fail:
		return "Fail"
	case structures.VerifyCallFailure:
		return "Verify call failure"
	case structures.VerifyClaimErrorAttempt:
		return "Verify claim error"
	case structures.NoResult:
		return "No result"
	case structures.MissingExpectedValue:
		return "Missing expected value"
	case structures.RuleCallFailure:
		return "Rule call failure"
	case structures.UnsetResultValue:
		return "Unset result value"
	default:
		return "Unknown"
	}
}

// Duration is the time between two timestamps, or "" if either is unset.
func Duration(from, to structures.Timestamp) string {
	return formatDuration(from, to)
}

// ShortTime formats a timestamp as "MM-DD HH:MM" UTC.
func ShortTime(t structures.Timestamp) string {
	return shortLabel(t)
}

// Dict builds a map from alternating keys and values, so that a partial
// template can be given more than one value, e.g.
// {{ template "x.html" (dict "Body" .Body "Scheme" "linux-uefi") }}.
func Dict(kv ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			m[k] = kv[i+1]
		}
	}
	return m
}

// PCRName returns the names of a register (e.g. PCR "7") in the naming scheme
// key, or nil if the scheme does not name it (#101). An empty or unknown key
// means the default scheme.
func PCRName(key string, register interface{}) *registernames.Register {
	return registernames.Lookup(key, fmt.Sprintf("%v", register))
}

// pcrSchemeView is the naming scheme a page uses, for the line saying so.
type pcrSchemeView struct {
	Key       string // the scheme used
	Title     string
	Requested string // what the element named, if anything
	Unknown   bool   // the element named a scheme that does not exist
}

// PCRScheme describes the scheme used for the element's key (#101).
func PCRScheme(key string) pcrSchemeView {
	s, known := registernames.Resolve(key)
	return pcrSchemeView{Key: s.Key, Title: s.Title, Requested: key, Unknown: key != "" && !known}
}

// SplitList splits a comma-separated value, e.g. a quote's pcrSelection
// "0,1,2,3", into its trimmed parts.
func SplitList(v interface{}) []string {
	var out []string
	for _, p := range strings.Split(fmt.Sprintf("%v", v), ",") {
		if p = strings.TrimSpace(p); p != "" && p != "<nil>" {
			out = append(out, p)
		}
	}
	return out
}
