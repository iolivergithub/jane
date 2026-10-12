package webui

import (
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"a10/configuration"
	"a10/operations"
	"a10/structures"
)

// The live page shows log entries as Jane publishes them over MQTT (topic AS/<channel>,
// one CSV line per entry). The browser only sees messages that arrive after it opens, so
// the page is seeded with the latest entries and with per-minute counts for the statistics
// window, and then keeps both up to date itself.

const (
	liveRecentEntries = 50                 // rows shown
	liveStatsWindow   = 3 * time.Hour      // period the statistics cover
	liveStatsDuration = "3h"               // the same, for operations.GetLogEntriesSince
	liveMinuteNanos   = int64(time.Minute) // statistics bucket size
)

// liveEntry is a log entry as the live page's script uses it.
type liveEntry struct {
	ItemID    string `json:"itemid"`
	Timestamp string `json:"ts"` // nanoseconds since the epoch, as a string so no precision is lost in JavaScript
	Channel   string `json:"channel"`
	Operation string `json:"op"`
	RefID     string `json:"refid"`
	RefType   string `json:"reftype"`
	Message   string `json:"message"`
	Element   string `json:"element,omitempty"` // element name, when the entry can be traced to one
}

// liveMinute holds the statistics for one minute (Minute = minutes since the epoch).
type liveMinute struct {
	Minute       int64 `json:"m"`
	Entries      int   `json:"n"`
	Attestations int   `json:"a"`
	Pass         int   `json:"p"`
	Fail         int   `json:"f"`
	Other        int   `json:"o"`
}

type livePage struct {
	Messaging   interface{}  // configuration.ConfigData.Messaging
	Recent      []liveEntry  // newest first
	Minutes     []liveMinute // only minutes with activity, oldest first
	WindowHours int
	MaxRows     int
}

func showLive(c echo.Context) error {
	p := livePage{
		Messaging:   configuration.ConfigData.Messaging,
		WindowHours: int(liveStatsWindow / time.Hour),
		MaxRows:     liveRecentEntries,
	}

	names := newElementNameCache()

	recent, _ := operations.GetLogEntries(liveRecentEntries)
	for _, e := range recent {
		p.Recent = append(p.Recent, liveEntry{
			ItemID: e.ItemID, Timestamp: strconv.FormatInt(int64(e.Timestamp), 10),
			Channel: e.Channel, Operation: e.Operation, RefID: e.RefID, RefType: e.RefType,
			Message: e.Message, Element: names.lookup(e.RefType, e.RefID),
		})
	}

	window, _ := operations.GetLogEntriesSince(liveStatsDuration)
	p.Minutes = liveMinutes(window)

	return c.Render(http.StatusOK, "live.html", p)
}

// liveMinutes counts log entries, attestations (claims added) and results by outcome per
// minute. entries may be in any order; minutes are returned oldest first, only those with
// activity.
func liveMinutes(entries []structures.LogEntry) []liveMinute {
	byMinute := map[int64]*liveMinute{}
	for _, e := range entries {
		m := int64(e.Timestamp) / liveMinuteNanos
		b, ok := byMinute[m]
		if !ok {
			b = &liveMinute{Minute: m}
			byMinute[m] = b
		}
		b.Entries++
		switch {
		case e.Channel == "C" && e.Operation == "add":
			b.Attestations++
		case e.Channel == "R" && e.Operation == "add":
			switch e.Message {
			case strconv.Itoa(int(structures.Success)):
				b.Pass++
			case strconv.Itoa(int(structures.Fail)):
				b.Fail++
			default:
				b.Other++
			}
		}
	}
	out := make([]liveMinute, 0, len(byMinute))
	for _, b := range byMinute {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Minute < out[j].Minute })
	return out
}

// liveElementName answers GET /live/element?reftype=...&refid=... with the name of the
// element a log entry refers to, so the live page can label entries that arrive over MQTT.
// It returns {"element": ""} when the entry cannot be traced to an element.
func liveElementName(c echo.Context) error {
	name := liveNames.lookup(c.QueryParam("reftype"), c.QueryParam("refid"))
	return c.JSON(http.StatusOK, map[string]string{"element": name})
}

// liveNames caches lookups for the /live/element endpoint across requests. Element names
// can change, so entries expire.
var liveNames = newElementNameCache()

type elementNameCache struct {
	mu      sync.Mutex
	entries map[string]cachedName
}

type cachedName struct {
	name    string
	expires time.Time
}

func newElementNameCache() *elementNameCache {
	return &elementNameCache{entries: map[string]cachedName{}}
}

// lookup returns the element name for an element, claim or result reference, or "".
func (n *elementNameCache) lookup(reftype, refid string) string {
	if refid == "" {
		return ""
	}
	key := reftype + "/" + refid
	n.mu.Lock()
	if c, ok := n.entries[key]; ok && time.Now().Before(c.expires) {
		n.mu.Unlock()
		return c.name
	}
	n.mu.Unlock()

	name := ""
	switch reftype {
	case "element":
		if e, err := operations.GetElementByItemID(refid); err == nil {
			name = e.Name
		}
	case "claim":
		if cl, err := operations.GetClaimByItemID(refid); err == nil {
			name = cl.Header.Element.Name
		}
	case "result":
		if r, err := operations.GetResultByItemID(refid); err == nil && r.ElementID != "" {
			if e, err := operations.GetElementByItemID(r.ElementID); err == nil {
				name = e.Name
			}
		}
	default:
		return ""
	}

	n.mu.Lock()
	if len(n.entries) > 5000 { // keep the cache bounded
		n.entries = map[string]cachedName{}
	}
	ttl := 10 * time.Minute
	if name == "" {
		ttl = 30 * time.Second // not found (yet, or deleted): don't remember that for long
	}
	n.entries[key] = cachedName{name: name, expires: time.Now().Add(ttl)}
	n.mu.Unlock()
	return name
}
