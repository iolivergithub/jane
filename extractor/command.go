package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// extractCommand runs one extraction, and the deletion if asked for.
func extractCommand(o cmdline) error {
	cfg, err := loadSettings(o)
	if err != nil {
		return err
	}
	if _, err := os.Stat(o.file); err == nil {
		return fmt.Errorf("%s exists; jp_extract never overwrites a file", o.file)
	}
	if o.delete {
		running, err := janeRunning(cfg, o.server)
		if err != nil {
			return err
		}
		if running != "" {
			fmt.Printf("WARNING: %s\n", running)
			if !confirm(os.Stdin, os.Stdout, "Jane is still running, continue? [yes/no] ") {
				return fmt.Errorf("stopped: nothing written or deleted")
			}
		}
	}

	db, err := openMongo(cfg.connection, cfg.database)
	if err != nil {
		return err
	}
	defer db.close()
	return extractWith(db, cfg, o)
}

// extractWith is the work after the database is open; the tests call it
// with a fake store. With --delete the caller has already checked whether
// Jane is running and asked.
func extractWith(s store, cfg settings, o cmdline) error {
	md := &metadata{
		Tool: "jp_extract", Version: VERSION, Build: BUILD, FormatVersion: formatVersion,
		Operation: "extract", Operator: operatorName(), System: cfg.system,
		Config: cfg.configFile, Database: cfg.database, Connection: redactConnection(cfg.connection),
		DocumentFormat: documentFormat,
	}
	md.Host, _ = os.Hostname()
	var err error
	if md.ExtractID, err = newUUID4(); err != nil {
		return err
	}

	md.Started = now()
	c, err := collect(s, o.elementID)
	if err != nil {
		return err
	}
	md.Finished = now()

	md.Element.ItemID, md.Element.Name = c.elementID, c.elementName
	md.Sessions.Exclusive = nonNil(c.exclusiveSessions)
	md.Sessions.Shared = nonNil(c.sharedSessions)
	md.Sessions.Children = nonNil(c.childSessions)
	md.MissingIntents = nonNil(c.missingIntents)

	if o.delete {
		md.Operation = "extract+delete"
		md.Delete = &deletePlan{Remove: map[string]int{}, RelationshipsRemoved: len(c.relationshipsTo),
			Kept: "intents; sessions listed as shared; the jane.log file and MQTT copies of log entries"}
		for _, coll := range deleteOrder {
			md.Delete.Remove[coll] = len(c.remove[coll])
		}
	}

	enc, err := encodeAll(c, md)
	if err != nil {
		return err
	}
	if err := writeFile(o.file, md, enc); err != nil {
		return fmt.Errorf("writing %s: %w", o.file, err)
	}
	if err := verifyFile(o.file, c, md); err != nil {
		return fmt.Errorf("the file did not read back correctly, nothing deleted: %w", err)
	}
	report(o.file, c, md)

	if !o.delete {
		return nil
	}
	return deleteClosure(s, c, md)
}

// deleteClosure removes what the file holds and --delete covers, in
// deleteOrder, with other elements' relationships to it (relationshipsTo)
// removed just before the element itself.
func deleteClosure(s store, c *closure, md *metadata) error {
	fmt.Printf("Deleting element %s (%s) from Jane\n", c.elementID, c.elementName)
	var problems []string
	for _, coll := range deleteOrder {
		if coll == "elements" && len(c.relationshipsTo) > 0 {
			n, err := s.pullRelationships(c.relatedElementIDs(), c.elementID)
			if err != nil {
				return fmt.Errorf("removing relationships to %s: %w", c.elementID, err)
			}
			fmt.Printf("  relationships     %6d elements changed\n", n)
			if want := len(c.relatedElementIDs()); int(n) != want {
				problems = append(problems, fmt.Sprintf("relationships: %d elements changed, expected %d", n, want))
			}
		}
		want := len(c.remove[coll])
		n, err := s.deleteIDs(coll, c.remove[coll])
		if err != nil {
			return fmt.Errorf("deleting %s (the file is complete; running again finishes the deletion): %w", coll, err)
		}
		fmt.Printf("  %-17s %6d deleted\n", coll, n)
		if int(n) != want {
			problems = append(problems, fmt.Sprintf("%s: %d deleted, expected %d", coll, n, want))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("deletion did not remove what was planned (is Jane running?): %s", strings.Join(problems, "; "))
	}
	return nil
}

func report(file string, c *closure, md *metadata) {
	fmt.Printf("Element %s (%s) extracted to %s\n", c.elementID, c.elementName, file)
	fmt.Printf("  extract ID %s\n", md.ExtractID)
	for _, coll := range collections {
		ci := md.Collections[coll]
		fmt.Printf("  %-17s %6d documents %10d bytes\n", coll, ci.Count, ci.Bytes)
	}
	for _, r := range c.relationshipsTo {
		fmt.Printf("  relationship: %s (%s) %s this element\n", r.ElementID, r.Name, r.RelationshipType)
	}
	if len(c.sharedSessions) > 0 {
		fmt.Printf("  %d session(s) also hold other elements' items; they are never deleted\n", len(c.sharedSessions))
	}
	if len(c.childSessions) > 0 {
		fmt.Printf("  %d session(s) not extracted have one of these sessions as parent\n", len(c.childSessions))
	}
	if len(c.missingIntents) > 0 {
		fmt.Printf("  %d intent(s) used are no longer in Jane: %s\n", len(c.missingIntents), strings.Join(c.missingIntents, ", "))
	}
}

// janeRunning says why Jane looks to be running, or "" if it does not:
// something listens on Jane's ports (from the configuration) or at the
// server URL, or systemd reports jane active. With nothing to check it says
// so, since Jane may then be running unseen.
func janeRunning(cfg settings, server string) (string, error) {
	addrs := append([]string(nil), cfg.listens...)
	if server != "" {
		a, err := serverAddress(server)
		if err != nil {
			return "", err
		}
		addrs = append(addrs, a)
	}
	if runtime.GOOS == "linux" {
		if path, err := exec.LookPath("systemctl"); err == nil {
			if exec.Command(path, "is-active", "--quiet", "jane").Run() == nil {
				return "systemd reports jane active", nil
			}
		}
	}
	for _, a := range addrs {
		conn, err := net.DialTimeout("tcp", a, 2*time.Second)
		if err == nil {
			conn.Close()
			return fmt.Sprintf("something is listening on %s, Jane's address", a), nil
		}
	}
	if len(addrs) == 0 {
		return "cannot tell whether Jane is running: no --config (Jane's config.yaml) or --server", nil
	}
	return "", nil
}

// confirm asks until it gets an answer: "yes" in full continues, an empty
// line asks again, anything else (or the end of the input) is no.
func confirm(in io.Reader, out io.Writer, question string) bool {
	r := bufio.NewReader(in)
	for {
		fmt.Fprint(out, question)
		line, err := r.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer == "" && err == nil {
			continue
		}
		if err != nil && answer == "" {
			fmt.Fprintln(out)
		}
		return answer == "yes"
	}
}

// serverAddress is host:port of a REST API URL.
func serverAddress(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("--server %q is not a URL such as http://jane:8520", server)
	}
	if u.Port() != "" {
		return u.Host, nil
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
