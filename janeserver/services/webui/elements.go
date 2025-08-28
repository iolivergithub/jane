package webui

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

type elementsStructure struct {
	E  structures.Element
	CS []structures.Claim
	RS []structures.Result
}

func showElements(c echo.Context) error {
	es, _ := operations.GetElementsAll()
	fmt.Printf("remdering element %v\n", len(es))

	return c.Render(http.StatusOK, "elements.html", es)
}

func showElement(c echo.Context) error {
	e, _ := operations.GetElementByItemID(c.Param("itemid"))

	fmt.Printf(" cparam is %v and e.ItemId is %v\n", c.Param("itemid"), e.ItemID)

	cs, _ := operations.GetClaimsByElementID(e.ItemID, 10)
	rs, _ := operations.GetResultsByElementID(e.ItemID, 10)

	fmt.Printf("showElement %v\n", c.Param("itemid"))

	es := elementsStructure{e, cs, rs}

	return c.Render(http.StatusOK, "element.html", es)
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
