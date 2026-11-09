// Command jp_extract copies one element out of Jane's database, with
// everything that belongs to it, into a single JSON file; with --delete it
// then removes those items from Jane.
//
//	jp_extract [options] ELEMENTID FILE
//
// It talks to MongoDB directly, as addjaneuser does: Jane's REST API has no
// way to read log entries by element and, by design, no way to delete
// claims, results or log entries.
package main

import (
	"fmt"
	"os"
	"strings"
)

const usage = `usage: jp_extract [-h] [--delete [--force]] [--config PATH]
                  [--mongodb URL] [--database NAME] [--server URL]
                  ELEMENTID FILE

Jane element extractor

Copies the element ELEMENTID out of Jane's database into FILE (JSON), with
its expected values, claims, results, sessions, log entries and the intents
it used. FILE has two sections: metadata (when, what, counts, sizes,
SHA-256 hashes) and extract (the documents, as MongoDB canonical Extended
JSON). FILE must not exist: jp_extract never overwrites a file.

positional arguments:
  ELEMENTID      the element's itemid
  FILE           the file to write

options:
  -h, --help     show this help message and exit
  --delete       after FILE is written and read back successfully, remove
                 the element, its expected values, claims, results, log
                 entries and the sessions holding only its claims and
                 results from Jane's database. Intents are never removed;
                 sessions shared with other elements stay. Jane must be
                 stopped
  --force        with --delete: also delete when other elements have a
                 relationship pointing at this element; those relationship
                 entries are removed from the other elements (they are
                 listed in FILE's metadata). Without --force jp_extract
                 refuses and writes nothing
  --config PATH  Jane's config.yaml, for the database connection and name
                 and the ports Jane listens on (default
                 /etc/opt/jane/config.yaml, if it exists)
  --mongodb URL  MongoDB connection string; overrides database: connection
  --database NAME
                 database name; overrides database: name
  --server URL   Jane's REST API, e.g. http://jane:8520 (else JANE_SERVER);
                 with --delete, jp_extract refuses if Jane answers there

environment:
  JANE_MONGODB_USERNAME, JANE_MONGODB_PASSWORD, JANE_MONGODB_AUTHSOURCE
                 MongoDB login, as for Jane (an unset or empty username
                 means no account); on a deb install they are in
                 /etc/opt/jane/jane.env
  JANE_SERVER    as --server

jp_extract writes nothing into Jane's log. With --delete it checks that
Jane is stopped: nothing may listen on Jane's REST, Web UI or x3270 ports
(from the configuration) or at --server, and on systemd "jane" must not be
active.

exit status:
  0  success
  1  error: element not found, database unreachable, FILE exists, Jane
     running (--delete), a deletion that did not remove what was planned
  2  bad command line
`

// VERSION and BUILD are set at build time by the Makefile, as for jp.
var VERSION string = "not set"
var BUILD string = "not set"

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// cmdline holds the command line options.
type cmdline struct {
	elementID string
	file      string
	delete    bool
	force     bool
	config    string
	mongodb   string
	database  string
	server    string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fmt.Printf("Jane Element Extractor (jp_extract version %s, build %s)\n", VERSION, BUILD)

	o, err := parseArgs(argv)
	if err == errHelp {
		fmt.Print(usage)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "jp_extract: %v\n%s", err, usage)
		return exitUsage
	}
	if err := extractCommand(o); err != nil {
		fmt.Fprintf(os.Stderr, "jp_extract: %v\n", err)
		return exitError
	}
	return exitOK
}

var errHelp = fmt.Errorf("help")

// parseArgs reads the command line; flags may appear anywhere, as for jp.
func parseArgs(argv []string) (cmdline, error) {
	var o cmdline
	var pos []string
	value := func(i *int, name string) (string, error) {
		a := argv[*i]
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v, nil
		}
		if *i+1 >= len(argv) {
			return "", fmt.Errorf("%s needs a value", name)
		}
		*i++
		return argv[*i], nil
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		var err error
		switch {
		case a == "-h" || a == "--help":
			return o, errHelp
		case a == "--delete":
			o.delete = true
		case a == "--force":
			o.force = true
		case a == "--config" || strings.HasPrefix(a, "--config="):
			o.config, err = value(&i, "--config")
		case a == "--mongodb" || strings.HasPrefix(a, "--mongodb="):
			o.mongodb, err = value(&i, "--mongodb")
		case a == "--database" || strings.HasPrefix(a, "--database="):
			o.database, err = value(&i, "--database")
		case a == "--server" || strings.HasPrefix(a, "--server="):
			o.server, err = value(&i, "--server")
		case strings.HasPrefix(a, "-") && a != "-":
			return o, fmt.Errorf("unknown option %s", a)
		default:
			pos = append(pos, a)
		}
		if err != nil {
			return o, err
		}
	}
	if len(pos) != 2 {
		return o, fmt.Errorf("ELEMENTID and FILE are both needed")
	}
	o.elementID, o.file = pos[0], pos[1]
	if o.elementID == "" {
		return o, fmt.Errorf("ELEMENTID is empty")
	}
	if o.force && !o.delete {
		return o, fmt.Errorf("--force only goes with --delete")
	}
	if o.server == "" {
		o.server = os.Getenv("JANE_SERVER")
	}
	return o, nil
}
