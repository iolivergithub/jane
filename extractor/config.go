package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// defaultConfig is where a deb install keeps Jane's configuration.
const defaultConfig = "/etc/opt/jane/config.yaml"

// janeConfig is the part of Jane's config.yaml jp_extract uses.
type janeConfig struct {
	System struct {
		Name string `yaml:"name"`
	} `yaml:"system"`
	Database struct {
		Connection string `yaml:"connection"`
		Name       string `yaml:"name"`
	} `yaml:"database"`
	Rest  listener `yaml:"rest"`
	Web   listener `yaml:"web"`
	X3270 listener `yaml:"x3270"`
}

type listener struct {
	Port     int    `yaml:"port"`
	ListenOn string `yaml:"listenon"`
}

// settings are what the command line and the configuration file give.
type settings struct {
	configFile string // empty if none was read
	system     string
	connection string
	database   string
	listens    []string // host:port Jane listens on, from the configuration
}

// loadSettings reads the configuration file (o.config, or the default if it
// exists) and applies --mongodb and --database over it.
func loadSettings(o cmdline) (settings, error) {
	var s settings
	path := o.config
	if path == "" {
		if _, err := os.Stat(defaultConfig); err == nil {
			path = defaultConfig
		}
	}
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return s, fmt.Errorf("configuration: %w", err)
		}
		var c janeConfig
		if err := yaml.Unmarshal(raw, &c); err != nil {
			return s, fmt.Errorf("configuration %s: %w", path, err)
		}
		s.configFile = path
		s.system = c.System.Name
		s.connection = c.Database.Connection
		s.database = c.Database.Name
		for _, l := range []listener{c.Rest, c.Web, c.X3270} {
			if l.Port > 0 {
				s.listens = append(s.listens, net.JoinHostPort(localAddress(l.ListenOn), strconv.Itoa(l.Port)))
			}
		}
	}
	if o.mongodb != "" {
		s.connection = o.mongodb
	}
	if o.database != "" {
		s.database = o.database
	}
	if s.connection == "" || s.database == "" {
		return s, fmt.Errorf("no database: give --config (Jane's config.yaml) or --mongodb and --database")
	}
	return s, nil
}

// localAddress is the address to try for a listener on listenon: a
// wildcard or empty listen address is reached on the loopback.
func localAddress(listenon string) string {
	switch listenon {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	}
	return listenon
}

// redactConnection removes any password from a MongoDB connection string,
// for the metadata and messages.
func redactConnection(conn string) string {
	u, err := url.Parse(conn)
	if err != nil {
		return "(unparsable connection string)"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}
