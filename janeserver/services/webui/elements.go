package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

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
	fmt.Printf("remdering element %v\n", len(es))

	cards := make([]elementCard, len(es))
	for i, e := range es {
		rs, _ := operations.GetResultsByElementID(e.ItemID, cardResultsFetched)
		cards[i] = elementCard{e, latestSessions(rs, cardSessions)}
	}

	return c.Render(http.StatusOK, "elements.html", cards)
}

func showElement(c echo.Context) error {
	e, _ := operations.GetElementByItemID(c.Param("itemid"))

	fmt.Printf(" cparam is %v and e.ItemId is %v\n", c.Param("itemid"), e.ItemID)

	cs, _ := operations.GetClaimsByElementID(e.ItemID, elementClaimsFetched)
	rs, _ := operations.GetResultsByElementID(e.ItemID, elementResultsFetched)

	fmt.Printf("showElement %v\n", c.Param("itemid"))

	page := buildElementPage(e, cs, rs, func(id string) (structures.Session, bool) {
		s, err := operations.GetSessionByItemID(id)
		return s, err == nil
	})

	return c.Render(http.StatusOK, "element.html", page)
}

func newElement(c echo.Context) error {
	fmt.Println("ELEMTEMPLATE is ", elementtemplate())

	return c.Render(http.StatusOK, "editelement.html", elementtemplate())
}

func processNewElement(c echo.Context) error {
	fmt.Println("\nProcessing New Element")
	elemdata := c.FormValue("elementdata")
	fmt.Println("ELEMDATA is ", elemdata)

	var newelem structures.Element

	err := json.Unmarshal([]byte(elemdata), &newelem)

	if err != nil {
		fmt.Printf("error is %v\n", err.Error())
		return c.Redirect(http.StatusSeeOther, "/new/element")
	}

	fmt.Printf("  fv%v\n", newelem)
	eid, err := operations.AddElement(newelem)
	fmt.Printf("  eid=%v,err=%v\n", eid, err)

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
