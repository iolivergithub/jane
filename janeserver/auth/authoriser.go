package auth

// Authoriser decides whether an authenticated user may perform a request.
//
// method is the HTTP method and path is Echo's route pattern (e.g. "/element/:itemid"),
// so a policy can be written against routes rather than concrete URLs.
//
// At present Jane only distinguishes logged in from not logged in, so the default
// policy allows everything. Role-based rules (e.g. read-only users who may not POST
// to /attest or /new/*) can be added later by implementing this interface and calling
// SetAuthoriser, without changing the middleware or any handler.
type Authoriser interface {
	Allowed(username, method, path string) bool
}

// AllowAuthenticated permits any authenticated user to do anything.
type AllowAuthenticated struct{}

func (AllowAuthenticated) Allowed(username, method, path string) bool { return username != "" }
