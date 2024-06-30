package restapi

import (
	"log"
	"net/http"

	"a10/operations"
	"a10/structures"

	"github.com/labstack/echo/v4"
)

type returnIntents struct {
	Intents []string `json:"intents"`
	Length  int      `json:"length"`
}

func getIntents(c echo.Context) error {
	elems, err := operations.GetIntents()

	log.Println("ps=", elems)

	if err != nil {
		log.Println("err=", err)
		return c.JSON(http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		//Convert elems from []structures.ID into a []string
		var elems_str []string
		for _, e := range elems {
			log.Println("--", e)
			elems_str = append(elems_str, e.ItemID)
		}

		//Marshall into JSON
		elems_struct := returnIntents{elems_str, len(elems_str)}

		return c.JSON(http.StatusOK, elems_struct)
	}
}

func getIntent(c echo.Context) error {
	itemid := c.Param("itemid")

	elem, err := operations.GetIntentByItemID(itemid)

	if err != nil {
		log.Println("err=", err)
		return c.JSON(http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		return c.JSON(http.StatusOK, elem)
	}
}

func getIntentsByName(c echo.Context) error {
	name := c.Param("name")

	elems, err := operations.GetIntentsByName(name)

	if err != nil {
		log.Println("err=", err)
		return c.JSON(http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		//Convert elems from []structures.ID into a []string
		var elems_str []string
		for _, e := range elems {
			elems_str = append(elems_str, e.ItemID)
		}

		//Marshall into JSON
		elems_struct := returnIntents{elems_str, len(elems_str)}

		return c.JSON(http.StatusOK, elems_struct)
	}
}

type postIntentReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

func postIntent(c echo.Context) error {
	elem := new(structures.Intent)

	if err := c.Bind(elem); err != nil {
		clienterr := postIntentReturn{"", err.Error()}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	res, err := operations.AddIntent(*elem)

	if err != nil {
		response := postIntentReturn{res, err.Error()}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		response := postIntentReturn{res, ""}
		return c.JSON(http.StatusCreated, response)
	}
}

func putIntent(c echo.Context) error {
	elem := new(structures.Intent)

	if err := c.Bind(elem); err != nil {
		clienterr := postIntentReturn{"", err.Error()}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	if _, err := operations.GetIntentByItemID(elem.ItemID); err != nil {
		response := postIntentReturn{"", err.Error()}
		return c.JSON(http.StatusNotFound, response)
	}

	log.Println("adding elemenet")
	err := operations.UpdateIntent(*elem)
	log.Println("creating response ", elem.ItemID, err)

	if err != nil {
		log.Println("err=", elem.ItemID)

		response := postIntentReturn{elem.ItemID, err.Error()}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		log.Println("res=", elem.ItemID)
		response := postIntentReturn{elem.ItemID, ""}
		return c.JSON(http.StatusCreated, response)
	}
}

func deleteIntent(c echo.Context) error {
	itemid := c.Param("itemid")

	log.Println("got here ", itemid)
	elem, err := operations.GetIntentByItemID(itemid)
	log.Println("Elem is ", elem)

	if err != nil {
		response := postIntentReturn{elem.ItemID, err.Error()}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		err = operations.DeleteIntent(itemid)
		if err != nil {
			response := postIntentReturn{itemid, err.Error()}
			return c.JSON(http.StatusInternalServerError, response)
		} else {
			response := postIntentReturn{itemid, ""}
			return c.JSON(http.StatusOK, response)
		}
	}
}
