package restapi

import (
	"errors"
	"net/http"

	"a10/operations"
	"a10/structures"

	"github.com/labstack/echo/v4"
)

// Element relationships (#118). These are the only way to change an
// element's relationships: PUT /element keeps the stored ones.
//
//	GET    /element/:itemid/relationships
//	POST   /element/:itemid/relationships                          {"relationshiptype": "hosted-on", "itemid": "<target>"}
//	DELETE /element/:itemid/relationships/:relationshiptype/:target

type returnElementRelationships struct {
	ItemID        string                           `json:"itemid"`
	Relationships []structures.ElementRelationship `json:"relationships"`
	Length        int                              `json:"length"`
	RelatedFrom   []operations.RelatedFrom         `json:"relatedfrom"` // other elements' relationships to this one
}

// relationshipStatus maps an operation error to an HTTP status.
func relationshipStatus(err error) int {
	switch {
	case errors.Is(err, operations.ErrorRelationshipType), errors.Is(err, operations.ErrorRelationshipSelf):
		return http.StatusBadRequest
	case errors.Is(err, operations.ErrorRelationshipExists):
		return http.StatusConflict
	case errors.Is(err, operations.ErrorItemNotFound), errors.Is(err, operations.ErrorRelationshipNotFound):
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func getElementRelationships(c echo.Context) error {
	itemid := c.Param("itemid")
	rels, err := operations.ListElementRelationships(itemid)
	if err != nil {
		return FormattedResponse(c, relationshipStatus(err), MakeRESTErrorMessage(err))
	}
	from, err := operations.GetElementsRelatedTo(itemid)
	if err != nil {
		return FormattedResponse(c, http.StatusInternalServerError, MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusOK, returnElementRelationships{itemid, rels, len(rels), from})
}

func postElementRelationship(c echo.Context) error {
	itemid := c.Param("itemid")
	rel := new(structures.ElementRelationship)
	if err := c.Bind(rel); err != nil {
		return FormattedResponse(c, http.StatusBadRequest, MakeRESTErrorMessage(err))
	}
	if err := operations.AddElementRelationship(itemid, *rel); err != nil {
		return FormattedResponse(c, relationshipStatus(err), MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusCreated, postElementReturn{itemid, ""})
}

func deleteElementRelationship(c echo.Context) error {
	itemid := c.Param("itemid")
	rel := structures.ElementRelationship{RelationshipType: c.Param("relationshiptype"), ItemID: c.Param("target")}
	if err := operations.DeleteElementRelationship(itemid, rel); err != nil {
		return FormattedResponse(c, relationshipStatus(err), MakeRESTErrorMessage(err))
	}
	return FormattedResponse(c, http.StatusOK, postElementReturn{itemid, ""})
}
