package webui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

// intentClaimsShown is how many recent claims the intent page lists.
const intentClaimsShown = 100

func showIntents(c echo.Context) error {
	is, _ := operations.GetIntentsAll()
	evs, _ := operations.GetExpectedValuesAll()
	return c.Render(http.StatusOK, "intents.html", buildIntentsPage(is, evs, operations.CountClaimsByIntentID))
}

func showIntent(c echo.Context) error {
	i, _ := operations.GetIntentByItemID(c.Param("itemid"))
	evs, _ := operations.GetExpectedValuesAll()

	cs := []structures.Claim{}
	total := int64(0)
	if i.ItemID != "" {
		cs, _ = operations.GetClaimsByIntentID(i.ItemID, intentClaimsShown)
		total = operations.CountClaimsByIntentID(i.ItemID)
	}
	return c.Render(http.StatusOK, "intent.html", buildIntentPage(i, evs, cs, total, dbLookups()))
}

func newIntent(c echo.Context) error {
	return c.Render(http.StatusOK, "editintent.html", intenttemplate())
}

func processNewIntent(c echo.Context) error {
	elemdata := c.FormValue("intentdata")

	var newIntent structures.Intent

	err := json.Unmarshal([]byte(elemdata), &newIntent)

	if err != nil {
		fmt.Printf("error is %v\n", err.Error())
		return c.Redirect(http.StatusSeeOther, "/new/intent")
	}

	fmt.Printf("  fv%v\n", newIntent)
	eid, err := operations.AddIntent(newIntent)
	fmt.Printf("  eid=%v,err=%v\n", eid, err)

	return c.Redirect(http.StatusSeeOther, "/intents")
}

// This is the template for an element
func intenttemplate() string {
	raw := `{ 
  "itemid" : "****", 
  "name" : "****", 
  "description" : "****", 
  "function" : "****",
  "parameters" : {}
 }
 `
	return raw
}

// Standard Intents
