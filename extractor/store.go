package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// store is everything jp_extract does to Jane's database: three operations,
// so the closure and deletion logic can be tested without MongoDB.
type store interface {
	// findIn returns the documents of coll whose field (dotted, possibly
	// through arrays) equals one of values.
	findIn(coll, field string, values []string) ([]bson.Raw, error)
	// deleteIDs removes the documents of coll with these _id values.
	deleteIDs(coll string, ids []bson.RawValue) (int64, error)
	// pullRelationships removes the relationship entries pointing at
	// target from the elements with the given itemids.
	pullRelationships(elementIDs []string, target string) (int64, error)
}

// chunk keeps $in lists well under MongoDB's 16 MB command limit.
const chunk = 1000

const opTimeout = 5 * time.Minute

type mongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

// openMongo connects as Jane does: the connection string from the
// configuration, credentials from JANE_MONGODB_* if the username is set.
func openMongo(conn, database string) (*mongoStore, error) {
	opts := options.Client().ApplyURI(conn)
	if user := os.Getenv("JANE_MONGODB_USERNAME"); user != "" {
		cred := options.Credential{}
		if opts.Auth != nil {
			cred = *opts.Auth
		}
		cred.Username, cred.Password, cred.PasswordSet = user, os.Getenv("JANE_MONGODB_PASSWORD"), true
		if src := os.Getenv("JANE_MONGODB_AUTHSOURCE"); src != "" {
			cred.AuthSource = src
		}
		opts.SetAuth(cred)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB %s: %w", redactConnection(conn), err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(context.Background())
		return nil, fmt.Errorf("MongoDB %s: %w", redactConnection(conn), err)
	}
	return &mongoStore{client: client, db: client.Database(database)}, nil
}

func (m *mongoStore) close() {
	m.client.Disconnect(context.Background())
}

func (m *mongoStore) findIn(coll, field string, values []string) ([]bson.Raw, error) {
	var out []bson.Raw
	for start := 0; start < len(values); start += chunk {
		part := values[start:min(start+chunk, len(values))]
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		cur, err := m.db.Collection(coll).Find(ctx, bson.M{field: bson.M{"$in": part}})
		if err != nil {
			cancel()
			return nil, fmt.Errorf("%s: %w", coll, err)
		}
		for cur.Next(ctx) {
			out = append(out, append(bson.Raw(nil), cur.Current...))
		}
		err = cur.Err()
		cur.Close(ctx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", coll, err)
		}
	}
	return out, nil
}

func (m *mongoStore) deleteIDs(coll string, ids []bson.RawValue) (int64, error) {
	var n int64
	for start := 0; start < len(ids); start += chunk {
		part := ids[start:min(start+chunk, len(ids))]
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		res, err := m.db.Collection(coll).DeleteMany(ctx, bson.M{"_id": bson.M{"$in": part}})
		cancel()
		if err != nil {
			return n, fmt.Errorf("%s: %w", coll, err)
		}
		n += res.DeletedCount
	}
	return n, nil
}

func (m *mongoStore) pullRelationships(elementIDs []string, target string) (int64, error) {
	if len(elementIDs) == 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	filter := bson.M{"itemid": bson.M{"$in": elementIDs}, "relationships.itemid": target}
	update := bson.M{"$pull": bson.M{"relationships": bson.M{"itemid": target}}}
	res, err := m.db.Collection("elements").UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("elements: %w", err)
	}
	return res.ModifiedCount, nil
}
