package operations

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"a10/datalayer"
	"a10/logging"
	"a10/structures"

	"go.mongodb.org/mongo-driver/bson"
)

// Element relationships (#118). A relationship is stored on its source
// element as (relationshiptype, itemid); the relationships form a set. They
// are changed only here: UpdateElement keeps the stored ones. No semantic
// checks beyond these: both elements exist, the type is known, and an
// element is not related to itself.

// checkRelationship checks a relationship for the element eid.
func checkRelationship(eid string, rel structures.ElementRelationship) error {
	if !slices.Contains(structures.RelationshipTypes, rel.RelationshipType) {
		return fmt.Errorf("%w %q (known: %s)", ErrorRelationshipType, rel.RelationshipType, strings.Join(structures.RelationshipTypes, ", "))
	}
	if rel.ItemID == eid {
		return ErrorRelationshipSelf
	}
	return nil
}

// AddElementRelationship adds rel to the element eid. Both elements must
// exist; adding a relationship already present gives ErrorRelationshipExists.
func AddElementRelationship(eid string, rel structures.ElementRelationship) error {
	if err := checkRelationship(eid, rel); err != nil {
		return err
	}
	if _, err := GetElementByItemID(eid); err != nil {
		return fmt.Errorf("element %s: %w", eid, err)
	}
	if _, err := GetElementByItemID(rel.ItemID); err != nil {
		return fmt.Errorf("related element %s: %w", rel.ItemID, err)
	}

	// $addToSet keeps the list a set, also when two adds overlap
	filter := bson.D{{"itemid", eid}}
	update := bson.D{{"$addToSet", bson.D{{"relationships", rel}}}}
	res, err := datalayer.DB.Collection("elements").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return ErrorItemNotFound
	}
	if res.ModifiedCount == 0 {
		return ErrorRelationshipExists
	}
	logging.MakeLogEntry("IM", "addrelationship", eid, "element", rel.RelationshipType+" "+rel.ItemID)
	return nil
}

// DeleteElementRelationship removes rel from the element eid.
func DeleteElementRelationship(eid string, rel structures.ElementRelationship) error {
	filter := bson.D{{"itemid", eid}}
	update := bson.D{{"$pull", bson.D{{"relationships", rel}}}}
	res, err := datalayer.DB.Collection("elements").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return ErrorItemNotFound
	}
	if res.ModifiedCount == 0 {
		return ErrorRelationshipNotFound
	}
	logging.MakeLogEntry("IM", "deleterelationship", eid, "element", rel.RelationshipType+" "+rel.ItemID)
	return nil
}

// ListElementRelationships returns the relationships of the element eid
// (never nil).
func ListElementRelationships(eid string) ([]structures.ElementRelationship, error) {
	e, err := GetElementByItemID(eid)
	if err != nil {
		return nil, err
	}
	if e.Relationships == nil {
		return []structures.ElementRelationship{}, nil
	}
	return e.Relationships, nil
}

// RelatedFrom is a relationship seen from its target: element ItemID (named
// Name) has a relationship of RelationshipType to the target.
type RelatedFrom struct {
	ItemID           string `json:"itemid"`
	Name             string `json:"name"`
	RelationshipType string `json:"relationshiptype"`
}

// GetElementsRelatedTo returns the relationships other elements have to the
// element itemid, e.g. the VMs hosted on a host.
func GetElementsRelatedTo(itemid string) ([]RelatedFrom, error) {
	var elems []structures.Element
	filter := bson.D{{"relationships.itemid", itemid}}
	dbcursor, err := datalayer.DB.Collection("elements").Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	if err := dbcursor.All(context.TODO(), &elems); err != nil {
		return nil, err
	}
	rtn := []RelatedFrom{}
	for _, e := range elems {
		for _, r := range e.Relationships {
			if r.ItemID == itemid {
				rtn = append(rtn, RelatedFrom{e.ItemID, e.Name, r.RelationshipType})
			}
		}
	}
	return rtn, nil
}
