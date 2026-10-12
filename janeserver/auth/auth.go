// Package auth provides authentication (who are you?) and authorisation (may you do this?)
// for Jane's Echo-based services.
//
// Everything to do with users, passwords, sessions and cookies lives in this package.
// The rest of the codebase touches it at only a few points:
//
//	janeserver.go                     auth.Initialise(configFile)
//	services/webui/initwebapi.go      auth.RegisterEndpoints(router); router.Use(auth.WebUIMiddleware())
//	services/restapi/initrestapi.go   router.Use(auth.RESTMiddleware())
//	services/webui/templates.go       "authEnabled": auth.WebEnabled   (shows the logout button)
//
// Configuration is read from an optional `auth:` section of the normal config.yaml.
// If that section is missing, authentication is disabled and Jane behaves exactly as before.
//
//	auth:
//	  web: true            # protect the Web UI
//	  rest: false          # protect the REST API with HTTP Basic auth
//	  sessiontimeout: 8h   # sliding expiry of a Web UI login
package auth

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"a10/configuration"
	"a10/logging"
)

// Config is the `auth:` section of config.yaml.
type Config struct {
	Web            bool          `yaml:"web"`
	Rest           bool          `yaml:"rest"`
	SessionTimeout time.Duration `yaml:"sessiontimeout"`
}

const defaultSessionTimeout = 8 * time.Hour

// Package state. Set once by Initialise (or by tests) before any service starts.
var (
	cfg        Config
	users      UserStore
	sessions   SessionStore
	authoriser Authoriser = AllowAuthenticated{}

	// logEntry writes to Jane's normal log (database, MQTT and file). It is a variable so tests can replace it.
	logEntry = logging.MakeLogEntry
)

// Initialise reads the `auth:` section of the configuration file and sets up the
// user store and session store. It must be called after the datalayer is initialised
// and before the Web UI or REST services are started.
//
// As with the main configuration, an unreadable or unparsable file causes a panic.
func Initialise(configFile string) {
	cfg = readConfig(configFile)

	users = newMongoUserStore()
	sessions = newMemorySessionStore(cfg.SessionTimeout)

	msg := fmt.Sprintf("Authentication: web %v, rest %v, session timeout %v", cfg.Web, cfg.Rest, cfg.SessionTimeout)
	fmt.Println("JANE: " + msg)
	logEntry("SYS", "startup/AUTH", configuration.ConfigData.System.Name, "auth", msg)

	if cfg.Web || cfg.Rest {
		if n, err := users.Count(); err != nil || n == 0 {
			warn := fmt.Sprintf("Authentication is enabled but the %q collection has no users (err=%v); nobody will be able to log in", usersCollection, err)
			fmt.Println("JANE: WARNING: " + warn)
			logEntry("SYS", "startup/AUTH", configuration.ConfigData.System.Name, "auth", warn)
		}
	}
}

// readConfig reads only the `auth:` section of the configuration file; the rest of the file
// is ignored, so the main configuration package needs no knowledge of authentication.
func readConfig(configFile string) Config {
	raw, err := os.ReadFile(configFile)
	if err != nil {
		panic(fmt.Sprintf("auth: unable to read configuration file %v: %v", configFile, err))
	}

	var wrapper struct {
		Auth *Config `yaml:"auth"`
	}
	if err := yaml.Unmarshal(raw, &wrapper); err != nil {
		panic(fmt.Sprintf("auth: unable to parse auth section of %v: %v", configFile, err))
	}

	c := Config{}
	if wrapper.Auth != nil {
		c = *wrapper.Auth
	}
	if c.SessionTimeout <= 0 {
		c.SessionTimeout = defaultSessionTimeout
	}
	return c
}

// WebEnabled reports whether the Web UI requires a login.
// It is exposed to the Web UI templates so that the logout button only appears when it means something.
func WebEnabled() bool { return cfg.Web }

// RESTEnabled reports whether the REST API requires credentials.
func RESTEnabled() bool { return cfg.Rest }

// SetAuthoriser replaces the authorisation policy. The default allows any authenticated user to do anything.
func SetAuthoriser(a Authoriser) { authoriser = a }
