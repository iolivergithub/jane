package datalayer

import (
	"context"
	"fmt"

	"a10/configuration"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Database
var DBCLIENT *mongo.Client
var ctx = context.TODO()

func initialiseDatabase() {
	fmt.Println("JANE: initialising database MONGO connection")

	clientOptions := options.Client().ApplyURI(configuration.ConfigData.Database.Connection)
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		panic(err.Error())
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		panic(err.Error())
	}

	DB = client.Database(configuration.ConfigData.Database.Name)

	fmt.Println("JANE: Database infrastructure MONGO is running")

	DBCLIENT = client

}
