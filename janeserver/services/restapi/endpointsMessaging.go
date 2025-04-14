package restapi

import (
	"log"

	"net/http"

	"a10/operations"
	"a10/structures"
)

import (
	"github.com/labstack/echo/v4"
)

type returnMessages struct {
	Messages []structures.Message `json:"messages"`
	Length   int                  `json:"length"`
}

func getMessagesForSession(c echo.Context) error {
	sid := c.Param("sid")

	messages, err := operations.GetMessagesForSession(sid)

	if err != nil {
		log.Println("err=", err)
		return FormattedResponse(c, http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		messages_struct := returnMessages{messages, len(messages)}
		return FormattedResponse(c, http.StatusOK, messages_struct)
	}
}

func getMessagesForElement(c echo.Context) error {
	eid := c.Param("eid")

	messages, err := operations.GetMessagesForElement(eid)

	if err != nil {
		log.Println("err=", err)
		return FormattedResponse(c, http.StatusInternalServerError, MakeRESTErrorMessage(err))
	} else {
		messages_struct := returnMessages{messages, len(messages)}
		return FormattedResponse(c, http.StatusOK, messages_struct)
	}
}
