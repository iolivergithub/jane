// This package contains the operations for managing elements in a system
// It provides
package operations

import (
	"context"

	"a10/datalayer"
	"a10/logging"
	"a10/structures"
	"a10/utilities"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CountIntents() int64 {
	return datalayer.Count("policies")
}

// AddIntent is a function that takes and element structure that has a BLANK Itemid field (empty string) and stores that
// element in some database
// Successful storage returns the itemid for that element and a nil error.
// An error is returned if an item id is given as part of the input structure.
func AddIntent(p structures.Intent) (string, error) {
	if p.ItemID != "" {
		return "", ErrorItemIDIncluded
	} else {
		p.ItemID = utilities.MakeID()
		_, dberr := datalayer.DB.Collection("intents").InsertOne(context.TODO(), p)
		logging.MakeLogEntry("IM", "add", p.ItemID, "intent", "")

		return p.ItemID, dberr
	}
}


// AddStandardIntent is a function that takes and element structure that has a FILLED IN Itemid field 
// and then replaces any existing version of that intent
func AddStandardIntent(stdintent structures.Intent)  {
    _,err := GetIntentByItemID(stdintent.ItemID)
    
    if err == nil {
    	dberr := UpdateIntent(stdintent)
    	if dberr == nil {
    		logging.MakeLogEntry("IM", "update", stdintent.ItemID, "stdintent", "update successful for "+stdintent.ItemID)
    	} else {
    		logging.MakeLogEntry("IM", "update", stdintent.ItemID, "stdintent", "update FAILED due to "+dberr.Error())
    	}
    	
    } else {
    	// we don't call AddIntent because it expects NO itemid and automatically generates one
    	// which is what we don't want here.
    	_, dberr := datalayer.DB.Collection("intents").InsertOne(context.TODO(), stdintent)
    	if dberr == nil {
    		logging.MakeLogEntry("IM", "add", stdintent.ItemID, "stdintent", "add successful for "+stdintent.ItemID)
    	} else {
    		logging.MakeLogEntry("IM", "add", stdintent.ItemID, "stdintent", "add FAILED due to "+dberr.Error())
    	}     
    }
}



// UpdateElement requires the complete structure, that is, it replaces the structure with the given itemid
func UpdateIntent(replacement structures.Intent) error {
	filter := bson.D{{"itemid", replacement.ItemID}}
	updateresult, err := datalayer.DB.Collection("intents").ReplaceOne(context.TODO(), filter, replacement)

	if err != nil {
		return err
	} else if updateresult.MatchedCount != 1 || updateresult.ModifiedCount != 1 {
		return ErrorItemNotUpdated
	} else {
		logging.MakeLogEntry("IM", "update", replacement.ItemID, "intent", "")

		return nil
	}
}

// DeleteElement takes an itemid as input
func DeleteIntent(itemid string) error {
	filter := bson.D{{"itemid", itemid}}
	deleteresult, err := datalayer.DB.Collection("intents").DeleteOne(context.TODO(), filter)

	if err != nil {
		return err
	} else if deleteresult.DeletedCount != 1 {
		return ErrorItemNotDeleted
	} else {
		logging.MakeLogEntry("IM", "delete", itemid, "intent", "")

		return nil
	}
}

// GetElements returns a map of itemids in the ID structure. If this structure is an empty map then no elements exist in the database.
func GetIntents() ([]structures.ID, error) {
	var elems []structures.ID

	filter := bson.D{{}} // Get all
	options := options.Find().SetProjection(bson.D{{"itemid", 1}}).SetSort(bson.D{{"name", 1}})
	dbcursor, _ := datalayer.DB.Collection("intents").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &elems)

	return elems, dbcursorerror
}

func GetIntentsAll() ([]structures.Intent, error) {
	var elems []structures.Intent

	filter := bson.D{{}} // Get all
	options := options.Find().SetSort(bson.D{{"name", 1}})
	dbcursor, _ := datalayer.DB.Collection("intents").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &elems)

	return elems, dbcursorerror
}

// GetElementByItemID returns a single element or error
func GetIntentByItemID(itemid string) (structures.Intent, error) {
	var pol structures.Intent

	// discard the cursor, it will be an empty entry if nothing exists
	filter := bson.D{{"itemid", itemid}}
	dbcursorerror := datalayer.DB.Collection("intents").FindOne(context.TODO(), filter).Decode(&pol)

	return pol,dbcursorerror
}

// GetElementByName returns all elements with the given name or an empty list.
func GetIntentsByName(name string) ([]structures.Intent, error) {
	var pol []structures.Intent

	// discard the error, the dbcursor.All will deal with that case
	filter := bson.D{{"name", name}}
	dbcursor, _ := datalayer.DB.Collection("intents").Find(context.TODO(), filter)
	dbcursorerror := dbcursor.All(context.TODO(), &pol)

	return pol, dbcursorerror
}
