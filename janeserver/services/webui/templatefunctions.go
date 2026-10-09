package webui

import (
	"fmt"
	"strconv"
	"time"

	"encoding/base64"
	"encoding/hex"

	"html/template"

	"a10/operations"
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
