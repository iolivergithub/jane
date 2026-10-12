package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"a10/datalayer"
)

// usersCollection is the Mongo collection holding user records.
//
// Users are created, and their passwords set, with the etc/addjaneuser script;
// Jane itself only reads this collection.
const usersCollection = "users"

// User is a record in the users collection.
// Password holds an Argon2id hash (see password.go), never the password itself.
type User struct {
	Username string `bson:"username"`
	Password string `bson:"password"`
}

// UserStore answers whether a username/password pair is valid.
type UserStore interface {
	Authenticate(username, password string) bool
	Count() (int64, error)
}

type mongoUserStore struct{}

func newMongoUserStore() UserStore { return mongoUserStore{} }

func (mongoUserStore) Authenticate(username, password string) bool {
	if username == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var u User
	err := datalayer.DB.Collection(usersCollection).FindOne(ctx, bson.D{{Key: "username", Value: username}}).Decode(&u)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			// Database trouble: fail closed.
			return false
		}
		// Unknown user: still verify against a real hash so that response time does not reveal which usernames exist.
		checkPassword(dummyHash, password)
		return false
	}
	return checkPassword(u.Password, password)
}

func (mongoUserStore) Count() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return datalayer.DB.Collection(usersCollection).CountDocuments(ctx, bson.D{})
}
