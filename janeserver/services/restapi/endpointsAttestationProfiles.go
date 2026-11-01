package restapi

import (
	"errors"
	"net/http"
	"strconv"

	"a10/auth"
	"a10/operations"
	"a10/structures"

	"github.com/labstack/echo/v4"
)

// Attestation profiles (#128, #129, #133).
//
//	PUT  /element/:itemid/attestationprofile/:name   {profile}  write it and make it the default (jp create/update)
//	POST /element/:itemid/runprov[/:name]            run the named profile, or the default
//	     ?parent=<sid>  ?followlimit=<n>  ?parallel=true
//
// Profiles are read with GET /element/:itemid. A run is synchronous: the
// answer is the report, with per-rule results and no verdict.

// profileStatus maps an operation error to an HTTP status.
func profileStatus(err error) int {
	switch {
	case errors.Is(err, operations.ErrorItemNotFound), errors.Is(err, operations.ErrorProfileNotFound), errors.Is(err, operations.ErrorNoDefault):
		return http.StatusNotFound
	case errors.Is(err, operations.ErrorProfileName), errors.Is(err, operations.ErrorProfileEmpty),
		errors.Is(err, operations.ErrorProfileStep), errors.Is(err, operations.ErrorProfileVariable),
		errors.Is(err, operations.ErrorProfileFollow):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func putAttestationProfile(c echo.Context) error {
	itemid, name := c.Param("itemid"), c.Param("name")
	p := new(structures.AttestationProfile)
	if err := c.Bind(p); err != nil {
		return FormattedResponse(c, http.StatusBadRequest, MakeRESTErrorMessage(err))
	}
	if err := operations.SetAttestationProfile(itemid, name, *p); err != nil {
		return FormattedResponse(c, profileStatus(err), MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusOK, postElementReturn{itemid, ""})
}

func postRunProfile(c echo.Context) error {
	itemid, name := c.Param("itemid"), c.Param("name")

	opts := operations.RunOptions{FollowLimit: operations.DefaultFollowLimit}
	if s := c.QueryParam("followlimit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return FormattedResponse(c, http.StatusBadRequest, restErrorMessage{"followlimit must be a non-negative integer"})
		}
		opts.FollowLimit = n
	}
	if s := c.QueryParam("parallel"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return FormattedResponse(c, http.StatusBadRequest, restErrorMessage{"parallel must be true or false"})
		}
		opts.Parallel = b
	}
	if name != "" && !operations.ValidProfileName(name) {
		return FormattedResponse(c, http.StatusBadRequest, MakeRESTErrorMessage(operations.ErrorProfileName))
	}

	e, err := operations.GetElementByItemID(itemid)
	if err != nil {
		return FormattedResponse(c, http.StatusNotFound, restErrorMessage{"Element " + itemid + " not found"})
	}
	pname, p, err := operations.ElementProfile(e, name)
	if err != nil {
		return FormattedResponse(c, profileStatus(err), MakeRESTErrorMessage(err))
	}

	user := auth.CurrentUser(c)
	if user == "" {
		user = "anonymous"
	}
	rc := operations.RunContext{User: user, Via: "REST", From: c.RealIP()}
	req, err := operations.StartProfileRun(e, pname, p, false, c.QueryParam("parent"), opts, rc)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, operations.ErrorItemNotFound) {
			status = http.StatusNotFound // the parent session
		}
		return FormattedResponse(c, status, MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusOK, req.Run())
}
