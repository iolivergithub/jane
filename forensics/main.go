// Command jp_forensics reads one element's history from Jane's REST API and
// writes it as a single, self-contained HTML timeline: the element, its
// expected values, the intents it used, its claims, results, sessions and log
// entries, with the differences between successive snapshots marked.
//
//	jp_forensics [options] ELEMENTID [--from TIME] [--until TIME]
//
// It only reads (HTTP GET): nothing in Jane is changed, and nothing is
// written into Jane's log by jp_forensics itself.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const usage = `usage: jp_forensics [-h] [--from TIME] [--until TIME] [--server URL]
                    [--apikeyfile PATH] [--out FILE] [--max N]
                    [--nodocuments] ELEMENTID

Jane element forensics

Reads the element ELEMENTID from Jane's REST API with its expected values,
the intents it used, its claims, results, sessions and log entries, and
writes them as a timeline in one HTML file (no external resources; open it
in any browser, offline). Every claim is compared with the one before it:
  - the element and intent copies it holds, with the previous claim's
  - its body, with the previous claim of the same intent and endpoint
    (e.g. which PCRs or UEFI variables changed, and when)
  - its endpoint, with the previous claim of the same intent and endpoint
and every result with the one before it for the same intent, endpoint and
rule (the outcome and the expected value copy it holds). The element,
intents and expected values as they are now are compared with their last
copies. The page can be searched and filtered, and shows each intent's
claims as a series and the history of every value that changed.

positional arguments:
  ELEMENTID      the element's itemid

options:
  -h, --help     show this help message and exit
  --from TIME    one end of the time range; if omitted, now
  --until TIME   the other end of the time range; if omitted, now
                 The range is between the two, whichever is earlier; with
                 neither, the element's whole history. TIME is
                   now
                   2026-11-17, 2026-11-17T09:30, 2026-11-17 09:30:00
                                   local time, unless a zone is given:
                   2026-11-17T09:30:00Z, 2026-11-17T09:30:00+02:00
                   90m, 36h, 7d, 2w   that long ago
                 A date alone, as the later end, means the end of that day
  --server URL   Jane's REST API, e.g. http://jane:8520 (else JANE_SERVER)
  --apikeyfile PATH
                 read the Jane REST API key from PATH (the key alone);
                 overrides JANE_API_KEY. Needed if Jane's REST API requires
                 a key (auth: rest: true)
  --out FILE     the HTML file to write (default
                 jp_forensics_<ELEMENTID>_<UTC time>.html); - writes to
                 standard output. An existing file is never overwritten
  --max N        the most claims, results and log entries asked of Jane
                 per list (default 100000); the report warns if a list
                 may have been cut short
  --nodocuments  leave the documents themselves (claims, results, ...) out
                 of the page: only the timeline and the differences

environment:
  JANE_SERVER    as --server
  JANE_API_KEY   Jane REST API key, jane_... (made with addjaneuser --apikey)

Claims and results just before the range are read too, as the baseline the
first ones in the range are compared with; they are not shown as events.

exit status:
  0  success
  1  error: Jane unreachable or refusing, element not found, FILE exists
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

const defaultMax = 100000

// cmdline holds the command line options.
type cmdline struct {
	elementID   string
	from        string
	until       string
	server      string
	apiKeyFile  *string
	out         string
	max         int64
	noDocuments bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fmt.Fprintf(os.Stderr, "Jane Element Forensics (jp_forensics version %s, build %s)\n", VERSION, BUILD)

	o, err := parseArgs(argv, os.Getenv)
	if err == errHelp {
		fmt.Print(usage)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "jp_forensics: %v\n%s", err, usage)
		return exitUsage
	}
	now := time.Now()
	tr, err := parseRange(o.from, o.until, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jp_forensics: %v\n%s", err, usage)
		return exitUsage
	}
	if err := forensicsCommand(o, tr, now); err != nil {
		fmt.Fprintf(os.Stderr, "jp_forensics: %v\n", err)
		return exitError
	}
	return exitOK
}

var errHelp = fmt.Errorf("help")

// parseArgs reads the command line; flags may appear anywhere, as for jp.
func parseArgs(argv []string, getenv func(string) string) (cmdline, error) {
	o := cmdline{max: defaultMax}
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
	is := func(a, name string) bool { return a == name || strings.HasPrefix(a, name+"=") }
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		var err error
		switch {
		case a == "-h" || a == "--help":
			return o, errHelp
		case a == "--nodocuments":
			o.noDocuments = true
		case is(a, "--from"):
			o.from, err = value(&i, "--from")
		case is(a, "--until"):
			o.until, err = value(&i, "--until")
		case is(a, "--server"):
			o.server, err = value(&i, "--server")
		case is(a, "--out"):
			o.out, err = value(&i, "--out")
		case is(a, "--apikeyfile"):
			var v string
			v, err = value(&i, "--apikeyfile")
			o.apiKeyFile = &v
		case is(a, "--max"):
			var v string
			if v, err = value(&i, "--max"); err == nil {
				o.max, err = strconv.ParseInt(v, 10, 64)
				if err != nil || o.max < 1 {
					err = fmt.Errorf("--max needs a positive number, not %q", v)
				}
			}
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("unknown option %s", a)
		default:
			pos = append(pos, a)
		}
		if err != nil {
			return o, err
		}
	}
	if len(pos) != 1 {
		return o, fmt.Errorf("exactly one ELEMENTID is needed")
	}
	o.elementID = pos[0]
	if o.elementID == "" {
		return o, fmt.Errorf("ELEMENTID is empty")
	}
	if o.server == "" {
		o.server = getenv("JANE_SERVER")
	}
	if o.server == "" {
		return o, fmt.Errorf("Jane's address is needed: give --server or set JANE_SERVER")
	}
	return o, nil
}
