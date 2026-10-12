package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"a10/datalayer"
)

// usersCollection is the Mongo collection holding user records.
//
// A user is created directly in the database for now, e.g. from mongosh:
//
//	db.users.insertOne({username: "ian", password: "secret"})
const usersCollection = "users"

// User is a record in the users collection.
// Passwords are stored in plain text for now; see checkPassword.
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
		// Unknown user: still do a comparison so that response time does not reveal which usernames exist.
		checkPassword("\x00no-such-user\x00", password)
		return false
	}
	return checkPassword(u.Password, password)
}

func (mongoUserStore) Count() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return datalayer.DB.Collection(usersCollection).CountDocuments(ctx, bson.D{})
}

// checkPassword compares a stored password with the one supplied.
//
// This is the only place that knows how passwords are stored. To move to hashed
// passwords (e.g. bcrypt), change this function and how users are created; nothing else changes.
func checkPassword(stored, given string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(given)) == 1
}
