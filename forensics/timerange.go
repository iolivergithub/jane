package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// timeRange is the period the report covers. All means no range was given:
// the element's whole history.
type timeRange struct {
	From, Until time.Time
	All         bool
}

// contains reports whether the Jane timestamp ts (Unix nanoseconds) is in r.
func (r timeRange) contains(ts int64) bool {
	if r.All {
		return true
	}
	return ts >= r.From.UnixNano() && ts <= r.Until.UnixNano()
}

// before reports whether ts is earlier than the range.
func (r timeRange) before(ts int64) bool {
	return !r.All && ts < r.From.UnixNano()
}

// after reports whether ts is later than the range.
func (r timeRange) after(ts int64) bool {
	return !r.All && ts > r.Until.UnixNano()
}

// includesNow reports whether the range reaches the present, so that the
// current element, intents and expected values belong in it.
func (r timeRange) includesNow(now time.Time) bool {
	return r.All || !r.Until.Before(now.Add(-time.Minute))
}

func (r timeRange) String() string {
	if r.All {
		return "the whole history"
	}
	return r.From.UTC().Format(time.RFC3339) + " – " + r.Until.UTC().Format(time.RFC3339)
}

// parseRange makes the range from --from and --until. A missing end is now;
// the range runs from the earlier to the later of the two, so the order they
// are given in does not matter. With neither, the range is everything.
func parseRange(from, until string, now time.Time) (timeRange, error) {
	if from == "" && until == "" {
		return timeRange{All: true}, nil
	}
	f, fDate, err := parseTime(from, now)
	if err != nil {
		return timeRange{}, fmt.Errorf("--from: %v", err)
	}
	u, uDate, err := parseTime(until, now)
	if err != nil {
		return timeRange{}, fmt.Errorf("--until: %v", err)
	}
	if u.Before(f) {
		f, u, uDate = u, f, fDate
	}
	if uDate {
		// a date alone as the later end: the whole of that day
		u = u.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	return timeRange{From: f, Until: u}, nil
}

var agoPattern = regexp.MustCompile(`^(\d+)(s|m|h|d|w)$`)

var timeLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// parseTime reads one TIME (see usage). An empty string is now. dateOnly is
// set for a date without a time.
func parseTime(s string, now time.Time) (t time.Time, dateOnly bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "now") {
		return now, false, nil
	}
	if m := agoPattern.FindStringSubmatch(s); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return t, false, fmt.Errorf("%q: %v", s, err)
		}
		unit := map[string]time.Duration{
			"s": time.Second, "m": time.Minute, "h": time.Hour,
			"d": 24 * time.Hour, "w": 7 * 24 * time.Hour,
		}[m[2]]
		return now.Add(-time.Duration(n) * unit), false, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, false, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, true, nil
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, now.Location()); err == nil {
			return t, false, nil
		}
	}
	return t, false, fmt.Errorf("%q is not a time: give now, a date such as 2026-11-17, a date and time such as 2026-11-17T09:30 or 2026-11-17T09:30:00Z, or a duration ago such as 36h or 7d", s)
}
