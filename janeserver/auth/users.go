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
// Users are created, and their passwords and API keys set, with the
// etc/addjaneuser/addjaneuser script; Jane itself only reads this collection.
const usersCollection = "users"

// User is a record in the users collection.
// Password holds an Argon2id hash (see password.go), never the password itself;
// it is empty for a user who only has an API key. APIKey holds "sha256:" and the
// hex SHA-256 of the user's REST API key (see restkeys.go), never the key itself;
// it is empty for a user without one. A user has at most one key.
type User struct {
	Username string `bson:"username"`
	Password string `bson:"password,omitempty"`
	APIKey   string `bson:"apikey,omitempty"`
}

// UserStore is the identity layer: the only code that reads the users collection.
// Another source of users (#48) would be another implementation of it.
type UserStore interface {
	// Authenticate reports whether a username/password pair is valid.
	Authenticate(username, password string) bool
	// UserForAPIKey returns the user whose API key has the given hash.
	UserForAPIKey(hash string) (username string, ok bool)
	// Exists reports whether a user still exists; err is set if that could not be found out.
	Exists(username string) (bool, error)
	// Count returns the number of users.
	Count() (int64, error)
	// CountAPIKeys returns the number of users with an API key.
	CountAPIKeys() (int64, error)
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
	if u.Password == "" {
		// A user with only an API key cannot log in with a password; take as long as a wrong one.
		checkPassword(dummyHash, password)
		return false
	}
	return checkPassword(u.Password, password)
}

func (mongoUserStore) UserForAPIKey(hash string) (string, bool) {
	if hash == "" {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var u User
	if err := datalayer.DB.Collection(usersCollection).FindOne(ctx, bson.D{{Key: "apikey", Value: hash}}).Decode(&u); err != nil {
		// Unknown key or database trouble: fail closed.
		return "", false
	}
	return u.Username, u.Username != ""
}

func (mongoUserStore) Exists(username string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := datalayer.DB.Collection(usersCollection).CountDocuments(ctx, bson.D{{Key: "username", Value: username}})
	return n > 0, err
}

func (mongoUserStore) CountAPIKeys() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return datalayer.DB.Collection(usersCollection).CountDocuments(ctx, bson.D{{Key: "apikey", Value: bson.D{{Key: "$gt", Value: ""}}}})
}

func (mongoUserStore) Count() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return datalayer.DB.Collection(usersCollection).CountDocuments(ctx, bson.D{})
}
