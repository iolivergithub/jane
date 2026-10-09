package x3270

import (
	"fmt"
	"os"

	"a10/configuration"
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

	CountLog() int64
	LogEntries(limit int64) ([]structures.LogEntry, error) // newest first
	System() systemInfo

	Rules() []structures.Rule
	Attest(req attestRequest) (sessionID string, err error)
}

// systemInfo is what the configuration screen shows (cf. the web home page).
type systemInfo struct {
	Config      *configuration.ConfigurationStruct
	CommandLine []string
	LogSize     int64 // bytes, -1 if the file can't be read

	Elements, Intents, ExpectedValues, Objects, Protocols, Rules int
	Sessions, Claims, Results, LogEntries                        int64
}

// attestRequest is everything needed to run one attestation, as collected
// by the attest screens (the same choices as the web attest form).
type attestRequest struct {
	ElementID, Endpoint, IntentID string
	Verify                        bool     // attest and verify, or attest only
	Rules                         []string // rule names, when verifying
	Message                       string
	IntentParams, RuleParams      map[string]interface{}
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

func (dbStore) CountLog() int64 { return operations.CountLogEntries() }
func (dbStore) LogEntries(limit int64) ([]structures.LogEntry, error) {
	return operations.GetLogEntries(limit)
}

func (dbStore) System() systemInfo {
	si := systemInfo{Config: configuration.ConfigData, CommandLine: os.Args, LogSize: -1}
	if es, err := operations.GetElements(); err == nil {
		si.Elements = len(es)
	}
	if is, err := operations.GetIntents(); err == nil {
		si.Intents = len(is)
	}
	if evs, err := operations.GetExpectedValues(); err == nil {
		si.ExpectedValues = len(evs)
	}
	if oos, err := operations.GetOpaqueObjects(); err == nil {
		si.Objects = len(oos)
	}
	si.Protocols = len(operations.GetProtocols())
	si.Rules = len(operations.GetRules())
	si.Sessions = operations.CountSessions()
	si.Claims = operations.CountClaims()
	si.Results = operations.CountResults()
	si.LogEntries = operations.CountLogEntries()
	if si.Config != nil {
		if fi, err := os.Stat(si.Config.Logging.LogFileLocation); err == nil {
			si.LogSize = fi.Size()
		}
	}
	return si
}

func (dbStore) Rules() []structures.Rule { return operations.GetRules() }

// Attest runs an attestation the same way as the web UI's attest form: open
// a session, collect the claim, apply each rule (unless attesting only) and
// close the session. The session is closed even if collecting the claim fails.
func (dbStore) Attest(req attestRequest) (string, error) {
	e, err := operations.GetElementByItemID(req.ElementID)
	if err != nil {
		return "", fmt.Errorf("element %s: %w", req.ElementID, err)
	}
	i, err := operations.GetIntentByItemID(req.IntentID)
	if err != nil {
		return "", fmt.Errorf("intent %s: %w", req.IntentID, err)
	}
	ips := req.IntentParams
	if ips == nil {
		ips = map[string]interface{}{}
	}
	rps := req.RuleParams
	if rps == nil {
		rps = map[string]interface{}{}
	}

	sid, err := operations.OpenSession(req.Message)
	if err != nil {
		return "", fmt.Errorf("could not open a session: %w", err)
	}
	defer operations.CloseSession(sid)
	s, err := operations.GetSessionByItemID(sid)
	if err != nil {
		return sid, fmt.Errorf("could not read session %s: %w", sid, err)
	}

	cid, err := operations.Attest(e, req.Endpoint, i, s, ips)
	if err != nil {
		return sid, fmt.Errorf("attestation failed: %w", err)
	}
	if !req.Verify {
		return sid, nil
	}

	cl, err := operations.GetClaimByItemID(cid)
	if err != nil {
		return sid, fmt.Errorf("could not read claim %s: %w", cid, err)
	}
	var missing []string
	for _, rn := range req.Rules {
		r, rerr := operations.GetRule(rn)
		if rerr != nil {
			missing = append(missing, rn)
			continue
		}
		operations.Verify(cl, r, s, rps)
	}
	if len(missing) > 0 {
		return sid, fmt.Errorf("unknown rules not applied: %v", missing)
	}
	return sid, nil
}
