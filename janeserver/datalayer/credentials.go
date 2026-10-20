package datalayer

import "os"

// Credentials for MongoDB and the MQTT broker come from environment variables
// (#79, #80), never from config.yaml. On a deb install systemd reads them from
// /etc/opt/jane/jane.env (EnvironmentFile=); anything else that sets a
// process's environment (a shell, a Windows service, Vault Agent, Kubernetes)
// works the same way. An unset or empty username means no account.
const (
	envMongoUser       = "JANE_MONGODB_USERNAME"
	envMongoPassword   = "JANE_MONGODB_PASSWORD"
	envMongoAuthSource = "JANE_MONGODB_AUTHSOURCE"
	envMQTTUser        = "JANE_MQTT_USERNAME"
	envMQTTPassword    = "JANE_MQTT_PASSWORD"
)

// getenv is os.Getenv; replaced in tests.
var getenv = os.Getenv

// envCredential returns the username and password held in the two variables.
// ok is false when the username is unset or empty: no account is used.
func envCredential(userVar, passVar string) (user, pass string, ok bool) {
	user = getenv(userVar)
	if user == "" {
		return "", "", false
	}
	return user, getenv(passVar), true
}

// What InitialiseDatalayer used, for the startup log (never the passwords).
var (
	mongoAuth = "none"
	mqttAuth  = "none"
)

// AuthSummary describes which accounts were used for MongoDB and MQTT, e.g.
// "MongoDB authentication: user jane; MQTT authentication: none".
func AuthSummary() string {
	return "MongoDB authentication: " + mongoAuth + "; MQTT authentication: " + mqttAuth
}
