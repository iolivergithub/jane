package webui

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"a10/logging"
	"a10/operations"
	"a10/structures"
)

// Number of most recent attestation sessions shown on each element card,
// and how many results are fetched per element to find them.
const cardSessions = 5
const cardResultsFetched = 250

// sessionResults is one row in an element card: the results of one session.
type sessionResults struct {
	SessionID  string
	VerifiedAt structures.Timestamp
	Results    []structures.Result
}

// elementCard is an element plus the results of its latest sessions.
type elementCard struct {
	structures.Element
	Sessions []sessionResults
}

// latestSessions groups an element's results (newest first) by session and
// returns at most n sessions, newest first, with results ordered by rule name
// so that the same rule sits in the same position on every row.
func latestSessions(rs []structures.Result, n int) []sessionResults {
	var out []sessionResults
	index := map[string]int{}

	for _, r := range rs {
		i, seen := index[r.Session.ItemID]
		if !seen {
			if len(out) == n {
				continue
			}
			i = len(out)
			index[r.Session.ItemID] = i
			out = append(out, sessionResults{SessionID: r.Session.ItemID, VerifiedAt: r.VerifiedAt})
		}
		out[i].Results = append(out[i].Results, r)
	}

	for i := range out {
		sort.SliceStable(out[i].Results, func(a, b int) bool {
			return out[i].Results[a].RuleName < out[i].Results[b].RuleName
		})
	}

	return out
}

func showElements(c echo.Context) error {
	es, _ := operations.GetElementsAll()

	cards := make([]elementCard, len(es))
	for i, e := range es {
		rs, _ := operations.GetResultsByElementID(e.ItemID, cardResultsFetched)
		cards[i] = elementCard{e, latestSessions(rs, cardSessions)}
	}

	return c.Render(http.StatusOK, "elements.html", cards)
}

func showElement(c echo.Context) error {
	e, _ := operations.GetElementByItemID(c.Param("itemid"))

	cs, _ := operations.GetClaimsByElementID(e.ItemID, elementClaimsFetched)
	rs, _ := operations.GetResultsByElementID(e.ItemID, elementResultsFetched)

	page := buildElementPage(e, cs, rs, func(id string) (structures.Session, bool) {
		s, err := operations.GetSessionByItemID(id)
		return s, err == nil
	})
	for _, r := range e.Relationships {
		rel := elementRelation{RelationshipType: r.RelationshipType, ItemID: r.ItemID}
		if t, err := operations.GetElementByItemID(r.ItemID); err == nil {
			rel.Name = t.Name
		} else {
			rel.Missing = true
		}
		page.Related = append(page.Related, rel)
	}
	page.RelatedFrom, _ = operations.GetElementsRelatedTo(e.ItemID)

	return c.Render(http.StatusOK, "element.html", page)
}

func newElement(c echo.Context) error {
	return c.Render(http.StatusOK, "editelement.html", elementtemplate())
}

func processNewElement(c echo.Context) error {
	elemdata := c.FormValue("elementdata")

	var newelem structures.Element

	err := json.Unmarshal([]byte(elemdata), &newelem)

	if err != nil {
		logging.MakeLogEntry("UI", "add", "", "element", "new element is not valid JSON: "+err.Error())
		return c.Redirect(http.StatusSeeOther, "/new/element")
	}

	if _, err := operations.AddElement(newelem); err != nil {
		logging.MakeLogEntry("UI", "add", "", "element", "adding the element failed: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/elements")
}

// This is the template for an element
func elementtemplate() string {
	raw := `{
    "name": "****",
    "description": "****",
    "endpoints":
    {
        "tarzan":
        {
            "endpoint": "http://127.0.0.1:8530",
            "protocol": "A10HTTPRESTv2"
        },
        "ratsd":
        {
            "endpoint": "http://127.0.0.1:8853",
            "protocol": "RATSD"
        }
    },
    "tags":
    [
        "****1",
        "****2"
    ],
    "host":
    {
        "os": "****",
        "arch": "****",
        "hostname": "****",
        "machineid": "****"
    },
    "tpm2":
    {
        "device": "/dev/tpmrm0",
        "ekcerthandle": "0x01c00002",
        "ek":
        {
            "handle": "0x810100EE",
            "public": "****"
        },
        "ak":
        {
            "handle": "0x810100AA",
            "public": "****"
        }
    },
    "uefi":
    {
        "eventlog": "/sys/kernel/security/tpm0/binary_bios_measurements"
    },
    "ima":
    {
        "asciilog": "/sys/kernel/security/ima/ascii_runtime_measurements"
    },
    "txt":
    {
        "log": ""
    }
}
	`
	return raw
}
