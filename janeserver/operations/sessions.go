// This package contains the operations for managing elements in a system
// It provides
package operations

import (
	"a10/configuration"
	"a10/datalayer"
	"a10/logging"
	"a10/structures"
	"a10/utilities"
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CountSessions() int64 {
	count, err := datalayer.DB.Collection("sessions").EstimatedDocumentCount(context.TODO())
	if err != nil {
		return -1
	} else {
		return count
	}
}

// AddElement is a function that takes and element structure that has a BLANK Itemid field (empty string) and stores that
// element in some database
// Successful storage returns the itemid for that element and a nil error.
// An error is returned if an item id is given as part of the input structure.
func OpenSession(msg string) (string, error) {
	// construct the session object
	t := structures.SessionTiming{utilities.MakeTimestamp(), 0}
	s := structures.Session{utilities.MakeID(), t, []string{}, []string{}, msg, structures.SessionFooter{}}
	// write to database
	_, dberr := datalayer.DB.Collection("sessions").InsertOne(context.TODO(), s)
	logging.MakeLogEntry("S", "open", s.ItemID, "session", msg)

	return s.ItemID, dberr
}

type hashablePartSession struct {
	ItemID     string
	Timing     structures.SessionTiming
	ClaimList  []string
	ResultList []string
	Message    string
}

// CloseSession closes the session and signs it.
//
// The footer covers the claim and result lists as read. The write only
// succeeds if the session is still open and its lists are unchanged (#90):
// if a claim or result was added in between, the session is read again and
// the close tried once more, so the signed lists always equal the stored ones.
// Anything added after the close is refused by AddClaimToSession and
// AddResultToSession.
func CloseSession(itemid string) error {
	for attempt := 0; attempt < 2; attempt++ {
		//Get the session, return error if not found
		session, err := GetSessionByItemID(itemid)
		if err != nil {
			return fmt.Errorf("Session not found %v : %v", itemid, err)
		}

		//Check if the session isn't already closed
		if session.Timing.Closed != 0 {
			return fmt.Errorf("Session already closed %v", itemid)
		}

		//The lists as read, for the filter below
		filter := bson.D{{"itemid", itemid}, {"timing.closed", 0}, {"claimlist", sameList(session.ClaimList)}, {"resultlist", sameList(session.ResultList)}}

		//Update the session
		session.Timing.Closed = utilities.MakeTimestamp()

		// hash and sign the session
		footer, _ := hashAndSignSession(hashablePartSession{session.ItemID, session.Timing, session.ClaimList, session.ResultList, session.Message})
		session.Footer = footer

		//Write to database
		closedSession, err := datalayer.DB.Collection("sessions").ReplaceOne(context.TODO(), filter, session)
		if err != nil {
			return fmt.Errorf("Database error in close session: %v", err)
		}
		if closedSession.MatchedCount == 1 {
			logging.MakeLogEntry("S", "close", itemid, "session", "")
			return nil
		}
		// No match: a claim or result was added (or the session closed) since it was read.
	}
	return fmt.Errorf("Session %v not closed: it changed while being closed; try again", itemid)
}

// sameList is the filter value matching a stored claim or result list equal to
// l. An empty list also matches one stored as null (older sessions).
func sameList(l []string) interface{} {
	if len(l) == 0 {
		return bson.D{{"$in", bson.A{nil, bson.A{}}}}
	}
	return l
}

// AddClaimToSession appends a claim to an open session. The append is a
// single atomic update that only matches an open session (#90), so
// overlapping adds are all kept and nothing is added after the close.
func AddClaimToSession(sid string, cid string) error {
	return addToSession(sid, "claimlist", cid, "claim")
}

// AddResultToSession appends a result to an open session; see AddClaimToSession.
func AddResultToSession(sid string, rid string) error {
	return addToSession(sid, "resultlist", rid, "result")
}

// addToSession appends id to the session's list field (claimlist or resultlist).
func addToSession(sid string, field string, id string, kind string) error {
	//Get the session, return error if not found
	_, err := GetSessionByItemID(sid)
	if err != nil {
		return fmt.Errorf("Session not found %v : %v", sid, err)
	}

	//ensure that the claim or result exists
	if kind == "claim" {
		_, err = GetClaimByItemID(id)
	} else {
		_, err = GetResultByItemID(id)
	}
	if err != nil {
		return fmt.Errorf("%v %v (for session) not found %v : %v", kind, id, sid, err)
	}

	//Append, only while the session is open
	filter := bson.D{{"itemid", sid}, {"timing.closed", 0}}
	update := bson.D{{"$push", bson.D{{field, id}}}}
	updatedsession, err := datalayer.DB.Collection("sessions").UpdateOne(context.TODO(), filter, update)

	if err != nil {
		return fmt.Errorf("Database error in add %v to session: %w", kind, err)
	} else if updatedsession.MatchedCount != 1 {
		return fmt.Errorf("Session %v is closed: %v %v not added", sid, kind, id)
	} else if updatedsession.ModifiedCount != 1 {
		return fmt.Errorf("Session %v not updated", sid)
	} else {
		if configuration.ConfigData.Logging.SessionUpdateLogging == true {
			logging.MakeLogEntry("S", "update", sid, "session", fmt.Sprintf("%s,%s", kind, id))
		}
		return nil
	}
}

// GetSessionByItemID returns a single session or error
func GetSessionByItemID(itemid string) (structures.Session, error) {
	var elem structures.Session

	// discard the cursor, it will be an empty entry if nothing exists
	filter := bson.D{{"itemid", itemid}}
	dbcursorerror := datalayer.DB.Collection("sessions").FindOne(context.TODO(), filter).Decode(&elem)

	if elem.ItemID == "" {
		return structures.Session{}, ErrorItemNotFound
	} else {
		return elem, dbcursorerror
	}
}

// GetElements returns a map of itemids in the ID structure. If this structure is an empty map then no elements exist in the database.
func GetSessions() ([]structures.SessionSummary, error) {
	var elems []structures.SessionSummary

	filter := bson.D{{}} // Get all
	options := options.Find().SetProjection(bson.D{{"itemid", 1}, {"timing.opened", 1}, {"timing.closed", 1}})
	dbcursor, _ := datalayer.DB.Collection("sessions").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &elems)

	return elems, dbcursorerror

}

func GetSessionsAll() ([]structures.Session, error) {
	var elems []structures.Session

	filter := bson.D{{}} // Get all
	options := options.Find().SetSort(bson.D{{"timing.opened", -1}})

	dbcursor, _ := datalayer.DB.Collection("sessions").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &elems)

	return elems, dbcursorerror

}
