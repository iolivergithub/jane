package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

type evstruct struct {
	EV structures.ExpectedValue
	E  structures.Element
	I  structures.Intent
}

// evResultsShown is how many recent results the expected value page lists.
const evResultsShown = 100

func showExpectedValues(c echo.Context) error {
	es, _ := operations.GetExpectedValuesAll()
	lk := dbLookups()

	evs := []evstruct{}
	for _, j := range es {
		evs = append(evs, evstruct{j, lk.element(j.ElementID), lk.intent(j.IntentID)})
	}
	sort.SliceStable(evs, func(a, b int) bool {
		if evs[a].E.Name != evs[b].E.Name {
			return evs[a].E.Name < evs[b].E.Name
		}
		return evs[a].EV.Name < evs[b].EV.Name
	})

	return c.Render(http.StatusOK, "evs.html", evs)
}

func showExpectedValue(c echo.Context) error {
	ev, _ := operations.GetExpectedValueByItemID(c.Param("itemid"))

	e, _ := operations.GetElementByItemID(ev.ElementID)
	i, _ := operations.GetIntentByItemID(ev.IntentID)

	rs := []structures.Result{}
	total := int64(0)
	if ev.ItemID != "" {
		rs, _ = operations.GetResultsByExpectedValueID(ev.ItemID, evResultsShown)
		total = operations.CountResultsByExpectedValueID(ev.ItemID)
	}

	return c.Render(http.StatusOK, "ev.html", buildEVPage(ev, e, i, rs, total))
}

type editevestruct struct {
	Elements []structures.ElementSummary
	Intents  []structures.Intent
}

func newExpectedValue(c echo.Context) error {

	e, _ := operations.GetElementsSummary()
	i, _ := operations.GetIntentsAll()

	evstr := editevestruct{e, i}

	return c.Render(http.StatusOK, "editexpectedvalue.html", evstr)
}

func processNewExpectedValue(c echo.Context) error {
	fmt.Println("\nProcessing New Element")
	itemid := c.FormValue("itemid")
	name := c.FormValue("name")
	description := c.FormValue("description")
	elementselect := c.FormValue("elementselect")
	intentselect := c.FormValue("intentselect")
	evsparameters := c.FormValue("evsparameters")

	// elementSelect is a CSV of itemid COMMA name of endpoint
	theelement := strings.Split(elementselect, ",")[0]
	endpointname := strings.Split(elementselect, ",")[1]

	var evsparams map[string]interface{}
	err := json.Unmarshal([]byte(evsparameters), &evsparams)
	if err != nil {
		fmt.Printf("error is %v\n", err.Error())
		return c.Redirect(http.StatusSeeOther, "/new/expectedvalue")
	}

	var newev = structures.ExpectedValue{itemid, name, description, theelement, endpointname, intentselect, evsparams, structures.RecordHistory{}}

	fmt.Printf("  fv%v\n", newev)
	eid, err := operations.AddExpectedValue(newev)
	fmt.Printf("  eid=%v,err=%v\n", eid, err)

	return c.Redirect(http.StatusSeeOther, "/expectedvalues")
}
