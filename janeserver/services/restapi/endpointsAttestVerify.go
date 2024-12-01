package restapi

import (
	"fmt"
	"net/http"

	"a10/operations"
	"a10/structures"

	"github.com/labstack/echo/v4"
)

type postAttestReturn struct {
	Itemid string `json:"itemid"`
	Error  string `json:"error"`
}

type postVerifyReturn struct {
	Itemid string                 `json:"itemid"`
	Result structures.ResultValue `json:"result"`
	Error  string                 `json:"error"`
}

type attestStr struct {
	EID        string                 `json:"eid"`
	PID        string                 `json:"pid"`
	SID        string                 `json:"sid"`
	Parameters map[string]interface{} `json:"parameters", bson:"parameters"`
}

type verifyStr struct {
	CID        string                 `json:"cid"`
	Rule       string                 `json:"rule"`
	SID        string                 `json:"sid"`
	Parameters map[string]interface{} `json:"parameters", bson:"parameters"`
}

func postAttest(c echo.Context) error {
	att := new(attestStr)
	if err := c.Bind(att); err != nil {
		clienterr := postAttestReturn{"", err.Error()}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	fmt.Printf("\n attstr is ###%v###", att)

	eid := (*att).EID
	pid := (*att).PID
	sid := (*att).SID

	element, err := operations.GetElementByItemID(eid)
	if err != nil {
		clienterr := postAttestReturn{"", "Element " + eid + " not found"}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	fmt.Printf("\n intent is ###%s###", pid)

	intent, err := operations.GetIntentByItemID(pid)
	if err != nil {
		clienterr := postAttestReturn{"", "Intent " + pid + " not found"}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	session, err := operations.GetSessionByItemID(sid)
	if err != nil {
		clienterr := postAttestReturn{"", "Session " + sid + " not found"}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	res, err := operations.Attest(element, intent, session, (*att).Parameters)

	if err != nil {
		response := postAttestReturn{res, err.Error()}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		response := postAttestReturn{res, ""}
		return c.JSON(http.StatusAccepted, response)
	}

}

func postVerify(c echo.Context) error {
	att := new(verifyStr)
	if err := c.Bind(att); err != nil {
		clienterr := postVerifyReturn{"", structures.VerifyCallFailure, err.Error()}
		return c.JSON(http.StatusBadRequest, clienterr)
	}

	cid := (*att).CID
	r := (*att).Rule
	sid := (*att).SID
	ps := (*att).Parameters

	claim, err := operations.GetClaimByItemID(cid)
	if err != nil {
		return fmt.Errorf("claim not found: %v", err)
	}

	rule, err := operations.GetRule(r)
	if err != nil {
		return fmt.Errorf("rule not found: %v", rule)
	}

	session, err := operations.GetSessionByItemID(sid)
	if err != nil {
		return fmt.Errorf("session not found: %v", err)
	}

	res, rv, err := operations.Verify(claim, rule, session, ps)

	if err != nil {
		response := postVerifyReturn{res, rv, err.Error()}
		return c.JSON(http.StatusInternalServerError, response)
	} else {
		response := postVerifyReturn{res, rv, ""}
		return c.JSON(http.StatusAccepted, response)
	}

}
