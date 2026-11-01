package datalayer

import (
	"context"
	"fmt"

	"a10/configuration"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Database
var DBCLIENT *mongo.Client
var ctx = context.TODO()

func initialiseDatabase() {
	fmt.Println("JANE: initialising database MONGO connection")

	clientOptions := options.Client().ApplyURI(configuration.ConfigData.Database.Connection)

	// Credentials from the environment (#79) replace any in the connection string.
	if user, pass, ok := envCredential(envMongoUser, envMongoPassword); ok {
		if clientOptions.Auth != nil && clientOptions.Auth.Username != "" {
			fmt.Println("JANE: the MongoDB connection string contains a username; " + envMongoUser + " is used instead")
		}
		// Keep anything else the connection string set (authSource, authMechanism).
		cred := options.Credential{}
		if clientOptions.Auth != nil {
			cred = *clientOptions.Auth
		}
		cred.Username, cred.Password, cred.PasswordSet = user, pass, true
		if src := getenv(envMongoAuthSource); src != "" {
			cred.AuthSource = src
		}
		clientOptions.SetAuth(cred)
		mongoAuth = "user " + user
	} else if clientOptions.Auth != nil && clientOptions.Auth.Username != "" {
		mongoAuth = "user " + clientOptions.Auth.Username + " (from the connection string)"
	}
	fmt.Println("JANE: MongoDB authentication: " + mongoAuth)
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		panic(err.Error())
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		panic(err.Error())
	}

	DB = client.Database(configuration.ConfigData.Database.Name)
	createIndexes()

	fmt.Println("JANE: Database infrastructure MONGO is running")

	DBCLIENT = client

}

// createIndexes makes the indexes Jane relies on (#130): sessions are looked
// up by item ID on every session page and by parent for the sessions opened
// under one. Creating an index that already exists does nothing. A failure is
// reported and Jane carries on: lookups still work, only more slowly.
func createIndexes() {
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "itemid", Value: 1}}},
		{Keys: bson.D{{Key: "parent", Value: 1}}},
	}
	if _, err := DB.Collection("sessions").Indexes().CreateMany(ctx, models); err != nil {
		fmt.Println("JANE: WARNING: could not create the indexes on sessions (itemid, parent):", err)
	}
}
