package x3270

// The log and configuration screens.

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/racingmars/go3270"

	"a10/structures"
)

// ---------------------------------------------------------------- log

// logList shows Jane's log, newest first. These are the same entries that
// are written to the log file (and to MQTT), read from the database so they
// can be paged.
func logList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Log",
		Header: fit("Time (UTC)", 14) + " " + fit("Ch", 2) + " " + fit("Operation", 10) + " " + fit("Type", 8) + " " + "Message",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			// the log is read newest first; fetch up to the end of this page
			es, err := s.LogEntries(skip + limit)
			if err != nil {
				return nil, 0, err
			}
			var items []listItem
			for _, e := range page(es, skip, limit) {
				e := e
				items = append(items, listItem{
					Text:   fmt.Sprintf("%s %s %s %s %s", shortWhen(e.Timestamp), fit(e.Channel, 2), fit(e.Operation, 10), fit(e.RefType, 8), e.Message),
					Detail: func() detail { return logDetail(e) },
				})
			}
			total := s.CountLog()
			if total < int64(len(es)) {
				total = int64(len(es))
			}
			return items, total, nil
		},
	}, back)
}

func logDetail(e structures.LogEntry) detail {
	d := detail{Title: "Log entry " + when(e.Timestamp)}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Time (UTC)", when(e.Timestamp))...)
	add(labelled("Channel", e.Channel)...)
	add(labelled("Operation", e.Operation)...)
	add(labelled("Refers to", strings.TrimSpace(e.RefType+" "+e.RefID))...)
	add(labelled("Item ID", e.ItemID)...)
	add(labelled("Message", e.Message)...)
	add(labelled("Hash", hex.EncodeToString(e.Hash))...)
	return d
}

// ---------------------------------------------------------------- configuration

// configScreen shows the attestation server's configuration and the number
// of each kind of record, like the web UI's home page.
func configScreen(s store, back go3270.Tx) go3270.Tx {
	return textTx(configDetail(s.System()), back)
}

// passwordInURL matches the password part of "scheme://user:password@host".
var passwordInURL = regexp.MustCompile(`(://[^:/@\s]*:)[^@/\s]*@`)

// maskPassword hides any password in a connection string.
func maskPassword(u string) string {
	return passwordInURL.ReplaceAllString(u, "${1}*****@")
}

func listenAddr(listenOn, port string, useHTTP bool) string {
	scheme := "https"
	if useHTTP {
		scheme = "http"
	}
	return fmt.Sprintf("%s:%s  (%s)", listenOn, port, scheme)
}

func configDetail(si systemInfo) detail {
	d := detail{Title: "Configuration"}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }

	add("Records")
	add("-------")
	add(fmt.Sprintf("%-16s %8d     %-16s %8d", "Elements", si.Elements, "Sessions", si.Sessions))
	add(fmt.Sprintf("%-16s %8d     %-16s %8d", "Intents", si.Intents, "Claims", si.Claims))
	add(fmt.Sprintf("%-16s %8d     %-16s %8d", "Expected values", si.ExpectedValues, "Results", si.Results))
	add(fmt.Sprintf("%-16s %8d     %-16s %8d", "Objects", si.Objects, "Log entries", si.LogEntries))
	add(fmt.Sprintf("%-16s %8d     %-16s %8d", "Protocols", si.Protocols, "Rules", si.Rules))

	add(section("Command line")...)
	add(labelled("Arguments", fmt.Sprintf("%d", len(si.CommandLine)))...)
	for _, a := range si.CommandLine {
		add(labelled("", a)...)
	}

	cfg := si.Config
	if cfg == nil {
		add("", "(configuration not loaded)")
		return d
	}

	add(section("System")...)
	add(labelled("Name", cfg.System.Name)...)

	add(section("Services")...)
	add(labelled("REST", listenAddr(cfg.Rest.ListenOn, cfg.Rest.Port, cfg.Rest.UseHTTP))...)
	add(labelled("Web", listenAddr(cfg.Web.ListenOn, cfg.Web.Port, cfg.Web.UseHTTP))...)
	add(labelled("X3270", ":"+cfg.X3270.Port)...)
	add(labelled("Keylime", or(cfg.Keylime.ApiUrl, "(not utilised)"))...)

	add(section("Logging")...)
	add(labelled("Log file", cfg.Logging.LogFileLocation)...)
	size := "(cannot read file)"
	if si.LogSize >= 0 {
		size = fmt.Sprintf("%d bytes", si.LogSize)
	}
	add(labelled("Size on disk", size)...)
	add(labelled("Entries", fmt.Sprintf("%d", si.LogEntries))...)
	sul := "disabled"
	if cfg.Logging.SessionUpdateLogging {
		sul = "enabled"
	}
	add(labelled("Session updates", sul)...)

	add(section("Database (MongoDB)")...)
	add(labelled("Name", cfg.Database.Name)...)
	add(labelled("Connection", maskPassword(cfg.Database.Connection))...)

	add(section("Message bus (MQTT)")...)
	add(labelled("Broker", cfg.Messaging.Broker)...)
	add(labelled("Port / WS port", fmt.Sprintf("%d / %d", cfg.Messaging.Port, cfg.Messaging.PortWS))...)
	add(labelled("Client ID", cfg.Messaging.ClientID)...)
	return d
}
