package auth

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type fakeUsers map[string]string

func (f fakeUsers) Authenticate(u, p string) bool {
	stored, ok := f[u]
	return ok && checkPassword(stored, p)
}
func (f fakeUsers) Count() (int64, error) { return int64(len(f)), nil }

// setup configures the package with fake users and returns routers shaped like Jane's Web UI and REST API.
func setup(t *testing.T, web, rest bool) (webRouter, restRouter *echo.Echo) {
	t.Helper()
	cfg = Config{Web: web, Rest: rest, SessionTimeout: time.Hour}
	// password "secret", hashed by etc/addjaneuser/addjaneuser
	users = fakeUsers{"ian": "$argon2id$v=19$m=19456,t=2,p=1$LqgRkKVdLySSZH7eD36sxg$ILzOqYRwfvBiYHJQq9tvrfqO/T+GtVWMpVNJYEGq49w"}
	sessions = newMemorySessionStore(time.Hour)
	authoriser = AllowAuthenticated{}
	logEntry = func(string, string, string, string, string) {}

	ok := func(c echo.Context) error { return c.String(http.StatusOK, "page for "+CurrentUser(c)) }

	webRouter = echo.New()
	RegisterEndpoints(webRouter)
	webRouter.Use(WebUIMiddleware())
	webRouter.GET("/", ok)
	webRouter.GET("/element/:itemid", ok)
	webRouter.POST("/attest", ok)

	restRouter = echo.New()
	restRouter.Use(RESTMiddleware())
	restRouter.GET("/elements", ok)
	restRouter.GET("/health", ok)
	return
}

func do(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, e *echo.Echo, user, pass, next string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"username": {user}, "password": {pass}, "next": {next}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(e, req)
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	return nil
}

func TestWebUnauthenticatedGetRedirectsToLogin(t *testing.T) {
	web, _ := setup(t, true, false)

	rec := do(web, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?next=%2F" {
		t.Fatalf("home: got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = do(web, httptest.NewRequest(http.MethodGet, "/element/abc?x=1", nil))
	if loc := rec.Header().Get("Location"); loc != "/login?next="+url.QueryEscape("/element/abc?x=1") {
		t.Fatalf("element: got %q", loc)
	}

	// Unknown routes are hidden too.
	rec = do(web, httptest.NewRequest(http.MethodGet, "/no/such/page", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unknown route: got %d", rec.Code)
	}
}

func TestWebUnauthenticatedPostIs401(t *testing.T) {
	web, _ := setup(t, true, false)
	rec := do(web, httptest.NewRequest(http.MethodPost, "/attest", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestLoginPageRenders(t *testing.T) {
	web, _ := setup(t, true, false)
	rec := do(web, httptest.NewRequest(http.MethodGet, "/login?next=%2Felements", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="next" value="/elements"`) {
		t.Fatalf("got %d\n%s", rec.Code, rec.Body.String())
	}
}

func TestLoginFailure(t *testing.T) {
	web, _ := setup(t, true, false)
	rec := login(t, web, "ian", "wrong", "/")
	if rec.Code != http.StatusUnauthorized || sessionCookie(rec) != nil {
		t.Fatalf("got %d, cookie %v", rec.Code, sessionCookie(rec))
	}
	if !strings.Contains(rec.Body.String(), "Incorrect username or password") {
		t.Fatal("error message missing")
	}
	if rec := login(t, web, "nobody", "secret", "/"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: got %d", rec.Code)
	}
}

func TestLoginSuccessThenAccessThenLogout(t *testing.T) {
	web, _ := setup(t, true, false)

	rec := login(t, web, "ian", "secret", "/element/abc")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/element/abc" {
		t.Fatalf("login: got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	ck := sessionCookie(rec)
	if ck == nil || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode {
		t.Fatalf("bad cookie %+v", ck)
	}

	req := httptest.NewRequest(http.MethodGet, "/element/abc", nil)
	req.AddCookie(ck)
	rec = do(web, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "page for ian" {
		t.Fatalf("authenticated get: %d %q", rec.Code, rec.Body.String())
	}

	// Already logged in: visiting /login goes straight on.
	req = httptest.NewRequest(http.MethodGet, "/login?next=%2Felement%2Fabc", nil)
	req.AddCookie(ck)
	if rec = do(web, req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/element/abc" {
		t.Fatalf("login when logged in: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(ck)
	rec = do(web, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("logout: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(ck)
	if rec = do(web, req); rec.Code != http.StatusSeeOther {
		t.Fatalf("old cookie still works after logout: %d", rec.Code)
	}
}

func TestBogusCookieRejected(t *testing.T) {
	web, _ := setup(t, true, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "made-up"})
	if rec := do(web, req); rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestOpenRedirectBlocked(t *testing.T) {
	for _, next := range []string{"https://evil.example", "//evil.example", "/\\evil.example", "", "javascript:alert(1)", "/logout"} {
		if got := safeNext(next); got != "/" {
			t.Errorf("safeNext(%q) = %q", next, got)
		}
	}
	if got := safeNext("/sessions?x=1"); got != "/sessions?x=1" {
		t.Errorf("local path rewritten: %q", got)
	}
}

func TestAuthoriserDenies(t *testing.T) {
	web, _ := setup(t, true, false)
	SetAuthoriser(readOnly{})
	ck := sessionCookie(login(t, web, "ian", "secret", "/"))

	req := httptest.NewRequest(http.MethodPost, "/attest", nil)
	req.AddCookie(ck)
	if rec := do(web, req); rec.Code != http.StatusForbidden {
		t.Fatalf("got %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(ck)
	if rec := do(web, req); rec.Code != http.StatusOK {
		t.Fatalf("read denied: %d", rec.Code)
	}
}

type readOnly struct{}

func (readOnly) Allowed(u, method, path string) bool { return method == http.MethodGet }

func TestWebDisabledIsPassThrough(t *testing.T) {
	web, _ := setup(t, false, false)
	if rec := do(web, httptest.NewRequest(http.MethodGet, "/", nil)); rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if rec := do(web, httptest.NewRequest(http.MethodGet, "/login", nil)); rec.Code != http.StatusSeeOther {
		t.Fatalf("login when disabled: %d", rec.Code)
	}
}

func TestREST(t *testing.T) {
	_, rest := setup(t, false, true)

	if rec := do(rest, httptest.NewRequest(http.MethodGet, "/elements", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no creds: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/elements", nil)
	req.SetBasicAuth("ian", "wrong")
	if rec := do(rest, req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad creds: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/elements", nil)
	req.SetBasicAuth("ian", "secret")
	if rec := do(rest, req); rec.Code != http.StatusOK || rec.Body.String() != "page for ian" {
		t.Fatalf("good creds: %d %q", rec.Code, rec.Body.String())
	}
	if rec := do(rest, httptest.NewRequest(http.MethodGet, "/health", nil)); rec.Code != http.StatusOK {
		t.Fatalf("health: %d", rec.Code)
	}
	if rec := do(rest, httptest.NewRequest(http.MethodOptions, "/elements", nil)); rec.Code == http.StatusUnauthorized {
		t.Fatal("CORS preflight blocked")
	}
}

func TestRESTDisabledIsPassThrough(t *testing.T) {
	_, rest := setup(t, false, false)
	if rec := do(rest, httptest.NewRequest(http.MethodGet, "/elements", nil)); rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestSessionExpiry(t *testing.T) {
	s := newMemorySessionStore(time.Minute)
	now := time.Now()
	s.now = func() time.Time { return now }
	tok, _ := s.Create("ian")

	now = now.Add(50 * time.Second)
	if _, ok := s.Lookup(tok); !ok {
		t.Fatal("expired too early")
	}
	now = now.Add(50 * time.Second) // sliding: still within a minute of last use
	if _, ok := s.Lookup(tok); !ok {
		t.Fatal("sliding expiry not applied")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := s.Lookup(tok); ok {
		t.Fatal("did not expire")
	}
}

func TestConfigParsing(t *testing.T) {
	dir := t.TempDir()
	parse := func(body string) Config {
		p := filepath.Join(dir, "c.yaml")
		os.WriteFile(p, []byte(body), 0o600)
		return readConfig(p)
	}
	if c := parse("system:\n  name: x\n"); c.Web || c.Rest || c.SessionTimeout != defaultSessionTimeout {
		t.Fatalf("missing section: %+v", c)
	}
	if c := parse("auth:\n  web: true\n  sessiontimeout: 30m\n"); !c.Web || c.Rest || c.SessionTimeout != 30*time.Minute {
		t.Fatalf("section: %+v", c)
	}
}

func TestTemplateFuncsBindPerRequest(t *testing.T) {
	setup(t, true, false)
	master := template.Must(template.New("p").Funcs(TemplateFuncs()).Parse(`{{ if authEnabled }}[{{ currentUser }}]{{ end }}`))

	render := func(user string) string {
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
		if user != "" {
			c.Set(contextUserKey, user)
		}
		page, err := master.Clone()
		if err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		if err := page.Funcs(RequestTemplateFuncs(c)).Execute(&sb, nil); err != nil {
			t.Fatal(err)
		}
		return sb.String()
	}
	if got := render("ian"); got != "[ian]" {
		t.Fatalf("got %q", got)
	}
	if got := render("<b>pekka</b>"); got != "[&lt;b&gt;pekka&lt;/b&gt;]" {
		t.Fatalf("username not escaped: %q", got)
	}
	cfg.Web = false
	if got := render("ian"); got != "" {
		t.Fatalf("shown when disabled: %q", got)
	}
}

func TestCurrentStatus(t *testing.T) {
	web, _ := setup(t, true, false)
	cfg.SessionTimeout = 90 * time.Minute
	login(t, web, "ian", "secret", "/")
	login(t, web, "ian", "secret", "/")
	st := CurrentStatus()
	if !st.Web || st.Rest || st.SessionTimeout != "1h30m" || st.Users != 1 || st.ActiveSessions != 2 || st.PasswordHash != "Argon2id" {
		t.Fatalf("got %+v", st)
	}
	for d, want := range map[time.Duration]string{8 * time.Hour: "8h", 30 * time.Minute: "30m", 90 * time.Second: "1m30s"} {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestAuthenticateForOtherInterfaces(t *testing.T) {
	setup(t, false, false)
	cfg.X3270 = true
	var logged []string
	logEntry = func(op, opname, item, itemtype, msg string) { logged = append(logged, opname+" "+item+" "+msg) }

	if !X3270Enabled() {
		t.Fatal("X3270Enabled() = false with x3270: true")
	}
	if Authenticate("ian", "wrong", "x3270", "10.0.0.5:1234") {
		t.Error("wrong password accepted")
	}
	if Authenticate("nobody", "secret", "x3270", "10.0.0.5:1234") {
		t.Error("unknown user accepted")
	}
	if !Authenticate("ian", "secret", "x3270", "10.0.0.5:1234") {
		t.Error("correct password rejected")
	}
	LogLogoff("ian", "x3270", "10.0.0.5:1234", "logoff")

	want := []string{
		"login/FAIL ian failed x3270 login from 10.0.0.5:1234",
		"login/FAIL nobody failed x3270 login from 10.0.0.5:1234",
		"login/OK ian x3270 login from 10.0.0.5:1234",
		"logout ian x3270 logoff from 10.0.0.5:1234",
	}
	if strings.Join(logged, "\n") != strings.Join(want, "\n") {
		t.Errorf("log entries:\n%s\nwant:\n%s", strings.Join(logged, "\n"), strings.Join(want, "\n"))
	}
}

func TestStartupWarnings(t *testing.T) {
	none, some := fakeUsers{}, fakeUsers{"ian": "x"}
	cases := []struct {
		name string
		c    Config
		u    UserStore
		want int
	}{
		{"all off", Config{}, none, 0},
		{"x3270 only, no users", Config{X3270: true}, none, 1},
		{"web without x3270", Config{Web: true}, some, 1},
		{"web and x3270", Config{Web: true, X3270: true}, some, 0},
		{"web, no users, no x3270", Config{Web: true}, none, 2},
	}
	for _, c := range cases {
		if got := startupWarnings(c.c, c.u); len(got) != c.want {
			t.Errorf("%s: %d warnings %q, want %d", c.name, len(got), got, c.want)
		}
	}
}

func TestReadConfigX3270(t *testing.T) {
	f := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(f, []byte("auth:\n  web: false\n  x3270: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if c := readConfig(f); !c.X3270 || c.Web || c.SessionTimeout != defaultSessionTimeout {
		t.Errorf("readConfig = %+v", c)
	}
}
