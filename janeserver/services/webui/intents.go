package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
    "reflect"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

const STDINTENTSURL="https://raw.githubusercontent.com/iolivergithub/jane/refs/heads/main/etc/standardintents/standardintents.json"
const STDINTENTSSHA="https://github.com/iolivergithub/jane/blob/main/etc/standardintents/standardintents.sha256"



func showIntents(c echo.Context) error {
	es, _ := operations.GetIntentsAll()
	return c.Render(http.StatusOK, "intents.html", es)
}

func showIntent(c echo.Context) error {
	e, _ := operations.GetIntentByItemID(c.Param("itemid"))
	return c.Render(http.StatusOK, "intent.html", e)
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




func loadstandardintents(c echo.Context) error {
    var intentfilestruct []structures.Intent

    resp,err := http.Get( STDINTENTSURL )

    if err != nil {
    	fmt.Println("getting std intents failed ", err.Error())
    	return err
    }

    defer resp.Body.Close()

    fmt.Println("Response ",resp.Status)

    jsonParser := json.NewDecoder(resp.Body)
    if err = jsonParser.Decode(&intentfilestruct); err != nil {
    	fmt.Println("parsing failed ",err.Error())
    }
    
    fmt.Println("parsing succeeded ",intentfilestruct)

    for i,e := range intentfilestruct {
    	fmt.Println(i,reflect.TypeOf(e))
        operations.AddIntent(e)
    }


    return nil

}