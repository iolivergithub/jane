package operations

import (
	"errors"
)

var (
	ErrorItemIDIncluded = errors.New("ItemID included")
	ErrorDatabaseError  = errors.New("Database Error")

	ErrorItemNotUpdated = errors.New("Item not updated")
	ErrorItemNotFound   = errors.New("Item not found")
	ErrorItemNotDeleted = errors.New("Element not deleted")

	// element relationships (#118)
	ErrorRelationshipType     = errors.New("unknown relationship type")
	ErrorRelationshipSelf     = errors.New("an element cannot be related to itself")
	ErrorRelationshipExists   = errors.New("relationship already exists")
	ErrorRelationshipNotFound = errors.New("relationship not found")
)
