package x3270

import (
	"a10/operations"
	"a10/structures"
)

// store is the data the 3270 screens read. The server uses dbStore; tests can
// supply their own.
type store interface {
	Elements() ([]structures.Element, error)
	Element(id string) (structures.Element, error)
	Intents() ([]structures.Intent, error)
	Intent(id string) (structures.Intent, error)
	ExpectedValues() ([]structures.ExpectedValue, error)
	ExpectedValue(id string) (structures.ExpectedValue, error)

	CountSessions() int64
	SessionsPage(skip, limit int64) ([]structures.Session, error)
	Session(id string) (structures.Session, error)

	CountClaims() int64
	ClaimsPage(skip, limit int64) ([]structures.Claim, error) // bodies not needed
	Claim(id string) (structures.Claim, error)                // with body
	ClaimsBySession(id string) ([]structures.Claim, error)

	CountResults() int64
	ResultsPage(skip, limit int64) ([]structures.Result, error)
	Result(id string) (structures.Result, error)
	ResultsBySession(id string) ([]structures.Result, error)
	ResultsByElement(id string, limit int64) ([]structures.Result, error)
}

// dbStore reads from Jane's database through the operations package.
type dbStore struct{}

func (dbStore) Elements() ([]structures.Element, error) { return operations.GetElementsAll() }
func (dbStore) Element(id string) (structures.Element, error) {
	return operations.GetElementByItemID(id)
}
func (dbStore) Intents() ([]structures.Intent, error) { return operations.GetIntentsAll() }
func (dbStore) Intent(id string) (structures.Intent, error) {
	return operations.GetIntentByItemID(id)
}
func (dbStore) ExpectedValues() ([]structures.ExpectedValue, error) {
	return operations.GetExpectedValuesAll()
}
func (dbStore) ExpectedValue(id string) (structures.ExpectedValue, error) {
	return operations.GetExpectedValueByItemID(id)
}

func (dbStore) CountSessions() int64 { return operations.CountSessions() }
func (dbStore) SessionsPage(skip, limit int64) ([]structures.Session, error) {
	return operations.GetSessionsPage(skip, limit)
}
func (dbStore) Session(id string) (structures.Session, error) {
	return operations.GetSessionByItemID(id)
}

func (dbStore) CountClaims() int64 { return operations.CountClaims() }
func (dbStore) ClaimsPage(skip, limit int64) ([]structures.Claim, error) {
	return operations.GetClaimsPage(skip, limit)
}
func (dbStore) Claim(id string) (structures.Claim, error) { return operations.GetClaimByItemID(id) }
func (dbStore) ClaimsBySession(id string) ([]structures.Claim, error) {
	return operations.GetClaimsBySessionIDs([]string{id})
}

func (dbStore) CountResults() int64 { return operations.CountResults() }
func (dbStore) ResultsPage(skip, limit int64) ([]structures.Result, error) {
	return operations.GetResultsPage(skip, limit)
}
func (dbStore) Result(id string) (structures.Result, error) {
	return operations.GetResultByItemID(id)
}
func (dbStore) ResultsBySession(id string) ([]structures.Result, error) {
	return operations.GetResultsBySessionIDs([]string{id})
}
func (dbStore) ResultsByElement(id string, limit int64) ([]structures.Result, error) {
	return operations.GetResultsByElementID(id, limit)
}
