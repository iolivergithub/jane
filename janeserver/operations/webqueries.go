package operations

// Queries used by the web UI's list and detail pages: paged listings (newest
// first) and lookups of claims and results by the session, claim, expected
// value or intent they belong to. Claim listings leave out the claim body,
// which can be large (e.g. IMA logs) and is not needed to list claims.

import (
	"context"

	"a10/datalayer"
	"a10/structures"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// claimWithoutBody excludes the claim body from a find.
var claimWithoutBody = bson.D{{"body", 0}}

// findInto runs a find on collection and decodes every document into out.
func findInto[T any](collection string, filter bson.D, opts *options.FindOptions, out *[]T) error {
	cursor, err := datalayer.DB.Collection(collection).Find(context.TODO(), filter, opts)
	if err != nil {
		return err
	}
	return cursor.All(context.TODO(), out)
}

// GetSessionsPage returns up to limit sessions after skipping skip, newest first.
func GetSessionsPage(skip, limit int64) ([]structures.Session, error) {
	var out []structures.Session
	opts := options.Find().SetSort(bson.D{{"timing.opened", -1}}).SetSkip(skip).SetLimit(limit)
	return out, findInto("sessions", bson.D{}, opts, &out)
}

// GetClaimsPage returns up to limit claims (without bodies) after skipping skip, newest first.
func GetClaimsPage(skip, limit int64) ([]structures.Claim, error) {
	var out []structures.Claim
	opts := options.Find().SetSort(bson.D{{"header.timing.requested", -1}}).SetSkip(skip).SetLimit(limit).SetProjection(claimWithoutBody)
	return out, findInto("claims", bson.D{}, opts, &out)
}

// GetResultsPage returns up to limit results after skipping skip, newest first.
func GetResultsPage(skip, limit int64) ([]structures.Result, error) {
	var out []structures.Result
	opts := options.Find().SetSort(bson.D{{"verifiedat", -1}}).SetSkip(skip).SetLimit(limit)
	return out, findInto("results", bson.D{}, opts, &out)
}

// GetResultsBySessionIDs returns every result belonging to any of the given sessions.
func GetResultsBySessionIDs(sids []string) ([]structures.Result, error) {
	var out []structures.Result
	if len(sids) == 0 {
		return out, nil
	}
	opts := options.Find().SetSort(bson.D{{"verifiedat", -1}})
	return out, findInto("results", bson.D{{"session.itemid", bson.D{{"$in", sids}}}}, opts, &out)
}

// GetClaimsBySessionIDs returns every claim (without bodies) belonging to any of the given sessions.
func GetClaimsBySessionIDs(sids []string) ([]structures.Claim, error) {
	var out []structures.Claim
	if len(sids) == 0 {
		return out, nil
	}
	opts := options.Find().SetSort(bson.D{{"header.timing.requested", -1}}).SetProjection(claimWithoutBody)
	return out, findInto("claims", bson.D{{"header.session.itemid", bson.D{{"$in", sids}}}}, opts, &out)
}

// GetResultsByClaimID returns the results produced by verifying one claim.
func GetResultsByClaimID(cid string) ([]structures.Result, error) {
	var out []structures.Result
	opts := options.Find().SetSort(bson.D{{"rulename", 1}})
	return out, findInto("results", bson.D{{"claimid", cid}}, opts, &out)
}

// GetResultsByExpectedValueID returns up to limit results that used an expected value, newest first.
func GetResultsByExpectedValueID(evid string, limit int64) ([]structures.Result, error) {
	var out []structures.Result
	opts := options.Find().SetSort(bson.D{{"verifiedat", -1}}).SetLimit(limit)
	return out, findInto("results", bson.D{{"expectedvalue.itemid", evid}}, opts, &out)
}

// GetClaimsByIntentID returns up to limit claims (without bodies) made with an intent, newest first.
func GetClaimsByIntentID(iid string, limit int64) ([]structures.Claim, error) {
	var out []structures.Claim
	opts := options.Find().SetSort(bson.D{{"header.timing.requested", -1}}).SetLimit(limit).SetProjection(claimWithoutBody)
	return out, findInto("claims", bson.D{{"header.intent.itemid", iid}}, opts, &out)
}

// CountClaimsByIntentID counts every claim made with an intent.
func CountClaimsByIntentID(iid string) int64 {
	n, err := datalayer.DB.Collection("claims").CountDocuments(context.TODO(), bson.D{{"header.intent.itemid", iid}})
	if err != nil {
		return -1
	}
	return n
}

// CountResultsByExpectedValueID counts every result that used an expected value.
func CountResultsByExpectedValueID(evid string) int64 {
	n, err := datalayer.DB.Collection("results").CountDocuments(context.TODO(), bson.D{{"expectedvalue.itemid", evid}})
	if err != nil {
		return -1
	}
	return n
}
