package restapi

import (
	"errors"
	"net/http"

	"a10/operations"
	"a10/structures"

	"github.com/labstack/echo/v4"
)

type returnSessions struct {
	Sessions []structures.SessionSummary `json:"sessions"`
	Length   int                         `json:"length"`
}

func getSessions(c echo.Context) error {
	elems, err := operations.GetSessions()

	if err != nil {
		return c.JSON(http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		//Convert elems from []structures.ID into a []string
		var elems_str []structures.SessionSummary
		for _, e := range elems {

			summary := structures.SessionSummary{e.ItemID, e.Timing}
			elems_str = append(elems_str, summary)
		}

		//Marshall into JSON
		elems_struct := returnSessions{elems_str, len(elems_str)}

		return FormattedResponse(c, http.StatusOK, elems_struct)
	}
}

func getSession(c echo.Context) error {
	itemid := c.Param("itemid")

	elem, err := operations.GetSessionByItemID(itemid)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		return c.JSON(http.StatusOK, elem)
	}
}

type returnSessionsByParent struct {
	Parent   string               `json:"parent"`
	Sessions []structures.Session `json:"sessions"`
	Length   int                  `json:"length"`
}

// getSessionsByParent lists the sessions opened under a session (#130).
func getSessionsByParent(c echo.Context) error {
	itemid := c.Param("itemid")
	elems, err := operations.GetSessionsByParent(itemid)
	if err != nil {
		return FormattedResponse(c, http.StatusInternalServerError, MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusOK, returnSessionsByParent{itemid, elems, len(elems)})
}

type postSessionMessage struct {
	Message string `json:"message"`
}

type postSessionReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

// POST causes a session to be opened
func postSession(c echo.Context) error {
	m := new(postSessionMessage)

	if err := c.Bind(m); err != nil {
		clienterr := postSessionReturn{"", err.Error()}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	// ?parent=<sid>: open it under that session (#130); the parent must exist
	res, err := operations.OpenSessionWithParent(m.Message, c.QueryParam("parent"), nil)

	if err != nil {
		response := postSessionReturn{res, err.Error()}
		if errors.Is(err, operations.ErrorItemNotFound) {
			return c.JSON(http.StatusNotFound, response) // no such parent
		}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		response := postSessionReturn{res, ""}
		return c.JSON(http.StatusCreated, response)
	}
}

// DELETE causes a session to be closed
func deleteSession(c echo.Context) error {
	itemid := c.Param("itemid")

	elem, err := operations.GetSessionByItemID(itemid)

	if err != nil {
		response := postSessionReturn{elem.ItemID, err.Error()}
		return c.JSON(http.StatusNotFound, response)
	} else {
		err = operations.CloseSession(itemid)
		if err != nil {
			response := postSessionReturn{itemid, err.Error()}
			return c.JSON(http.StatusInternalServerError, response)
		} else {
			response := postSessionReturn{itemid, ""}
			return c.JSON(http.StatusOK, response)
		}
	}
}

func putSessionClaim(c echo.Context) error {
	sid := c.Param("sid")
	cid := c.Param("cid")

	elem, err := operations.GetSessionByItemID(sid)

	if err != nil {
		response := postSessionReturn{elem.ItemID, err.Error()}
		return c.JSON(http.StatusNotFound, response)
	} else {
		err = operations.AddClaimToSession(sid, cid)
		if err != nil {
			response := postSessionReturn{sid, err.Error()}
			return c.JSON(http.StatusInternalServerError, response)
		} else {
			response := postSessionReturn{sid, ""}
			return c.JSON(http.StatusOK, response)
		}
	}
}

func putSessionResult(c echo.Context) error {
	sid := c.Param("sid")
	rid := c.Param("rid")

	elem, err := operations.GetSessionByItemID(sid)

	if err != nil {
		response := postSessionReturn{elem.ItemID, err.Error()}
		return c.JSON(http.StatusNotFound, response)
	} else {
		err = operations.AddResultToSession(sid, rid)
		if err != nil {
			response := postSessionReturn{sid, err.Error()}
			return c.JSON(http.StatusInternalServerError, response)
		} else {
			response := postSessionReturn{sid, ""}
			return c.JSON(http.StatusOK, response)
		}
	}
}
