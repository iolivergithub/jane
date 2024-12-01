package webui

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"a10/operations"
	"a10/structures"
)

func showOpaqueObjects(c echo.Context) error {
	os, _ := operations.GetOpaqueObjects()

	return c.Render(http.StatusOK, "opaqueobjects.html", os)
}

func showOpaqueObject(c echo.Context) error {
	o, _ := operations.GetOpaqueObjectByValue(c.Param("name"))

	return c.Render(http.StatusOK, "opaqueobject.html", o)
}

func newOpaqueObject(c echo.Context) error {

	return c.Render(http.StatusOK, "editopaqueobject.html", nil)
}

func processOpaqueObject(c echo.Context) error {
	fmt.Println("\nProcessing New Opaque Object")
	value := c.FormValue("value")
	otype := c.FormValue("type")
	shortdescription := c.FormValue("shortdescription")
	longdescription := c.FormValue("longdescription")

	var newoo = structures.OpaqueObject{value, otype, shortdescription, longdescription}

	eid, err := operations.AddOpaqueObject(newoo)
	fmt.Printf("  eid=%v,err=%v\n", eid, err)

	return c.Redirect(http.StatusSeeOther, "/opaqueobjects")
}
