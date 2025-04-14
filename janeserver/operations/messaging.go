// This package contains the operations for managing elements in a system
// It provides
package operations

import (
	"context"
	"fmt"

	"a10/datalayer"
	"a10/logging"
	"a10/structures"
	"a10/utilities"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CountMessages() int64 {
	return datalayer.Count("messages")
}

func AddMessage(sid string, eid string, msg string) error {
	//Check the session exists, return error if not found
	session, err := GetSessionByItemID(sid)
	if err != nil {
		return fmt.Errorf("Session %v not found: %v", sid, err)
	}

	//Check if the session isn't already closed
	if session.Timing.Closed != 0 {
		return fmt.Errorf("Session %v already closed: %v", sid, err)
	}

	_, err = GetElementByItemID(eid)
	if err == ErrorItemNotFound {
		return fmt.Errorf("Element %v does not exist: %v", eid, err)
	}

	mid := utilities.MakeID()
	ts := utilities.MakeTimestamp()

	message := structures.Message{
		ItemID:    mid,
		SessionID: sid,
		ElementID: eid,
		Contents:  msg,
		Timestamp: ts,
	}

	_, dberr := datalayer.DB.Collection("messages").InsertOne(context.TODO(), message)
	logging.MakeLogEntry("MSG", "add", mid, "message", msg)

	return dberr

}

func GetMessagesByItemID(itemid string) (structures.Message, error) {
	var msg structures.Message

	// discard the cursor, it will be an empty entry if nothing exists
	filter := bson.D{{"itemid", itemid}}
	dbcursorerror := datalayer.DB.Collection("messages").FindOne(context.TODO(), filter).Decode(&msg)

	if msg.ItemID == "" {
		return structures.Message{}, ErrorItemNotFound
	} else {
		return msg, dbcursorerror
	}
}

func GetMessagesForElement(eid string) ([]structures.Message, error) {
	var msgs []structures.Message

	options := options.Find().SetSort(bson.D{{"timestamp", -1}})
	filter := bson.D{{"elementid", eid}}
	dbcursor, _ := datalayer.DB.Collection("messages").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &msgs)

	return msgs, dbcursorerror
}

func GetMessagesForSession(sid string) ([]structures.Message, error) {
	var msgs []structures.Message

	options := options.Find().SetSort(bson.D{{"timestamp", -1}})
	filter := bson.D{{"sessionid", sid}}
	dbcursor, _ := datalayer.DB.Collection("messages").Find(context.TODO(), filter, options)
	dbcursorerror := dbcursor.All(context.TODO(), &msgs)

	return msgs, dbcursorerror
}
