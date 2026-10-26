// Command jp is the Jane element provisioner: it registers this machine as
// an element with a Jane attestation server and, optionally, provisions the
// TPM and records expected values; it can then attest the element.
//
//	jp create|update|attest <provisioning file> [--reprovisiontpm] [--recreate] [--apikeyfile PATH]
package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const usage = `usage: jp [-h] [--reprovisiontpm] [--recreate] [--apikeyfile PATH] operation pfile

Jane Element Configurator

positional arguments:
  operation      create   register this machine as a new element; refuses if it
                          is already registered (element ID file present,
                          or an element in Jane with the same name or
                          machine ID)
                 update   update the element registered earlier
                 attest   attest the registered element against every intent
                          and rule under evs, without changing anything
  pfile          provisioning file (YAML)

options:
  -h, --help     show this help message and exit
  --reprovisiontpm
                 create and update only: run the worklist's destructive TPM
                 steps, tpmclear (evict the keys at the EK/AK handles) and
                 tpmprovision (create new EK and AK). Without it they are
                 skipped and the TPM is left unchanged, so jp can run
                 unattended safely. attest never touches the TPM
  --recreate     create only: register a new element even though this
                 machine is already registered; the old element is left in
                 Jane, orphaned
  --apikeyfile PATH
                 read the Jane REST API key from PATH (the key alone, e.g.
                 /etc/opt/jane/jp.key); overrides JANE_API_KEY. Needed if
                 Jane's REST API requires a key (auth: rest: true)

environment:
  JANE_API_KEY   Jane REST API key, jane_... (sudo -E keeps it); keys are
                 made with addjaneuser --apikey
  JANE_TPM_ENDORSEMENT_PASSWORD
                 the TPM's endorsement hierarchy password, if one is set
  JANE_TPM_OWNER_PASSWORD
                 the TPM's owner hierarchy password, if one is set (used to
                 make the EK and AK persistent and to evict them)

jp never asks questions: anything destructive needs its own flag
(--reprovisiontpm, --recreate). create and update need root (the TPM and
/etc/opt/jane/janeelementid); attest does not.

exit status:
  0  success (attest: every rule passed)
  1  error: bad provisioning file, Jane server or TPM unreachable, ...
  2  bad command line
  3  attest only: a rule did not pass or an intent could not be attested
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fmt.Println("Jane Element Configuration")

	// Flags may appear anywhere, as with the original argparse CLI.
	reprovision, recreate := false, false
	var keyFile *string
	var pos []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--reprovisiontpm":
			reprovision = true
		case a == "--recreate":
			recreate = true
		case a == "-h" || a == "--help":
			fmt.Print(usage)
			return exitOK
		case a == "--user" || a == "--pass" || strings.HasPrefix(a, "--user=") || strings.HasPrefix(a, "--pass="):
			fmt.Fprintf(os.Stderr, "jp: --user and --pass were removed in 1.5.5: Jane's REST API uses API keys; set JANE_API_KEY or give --apikeyfile\n%s", usage)
			return exitUsage
		case a == "--apikeyfile" || strings.HasPrefix(a, "--apikeyfile="):
			name, val, hasVal := strings.Cut(a, "=")
			if !hasVal {
				if i+1 >= len(argv) {
					fmt.Fprintf(os.Stderr, "jp: %s needs a value\n%s", name, usage)
					return exitUsage
				}
				i++
				val = argv[i]
			}
			keyFile = &val
		default:
			if len(a) > 1 && a[0] == '-' {
				fmt.Fprintf(os.Stderr, "jp: unknown option %s\n%s", a, usage)
				return exitUsage
			}
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	op, pfile := pos[0], pos[1]

	if op != "create" && op != "update" && op != "attest" {
		fmt.Println("Unknown command, not one of: create, update, attest")
		return exitUsage
	}

	if recreate && op != "create" {
		fmt.Fprintf(os.Stderr, "jp: --recreate only applies to create\n%s", usage)
		return exitUsage
	}
	if reprovision && op == "attest" {
		fmt.Fprintf(os.Stderr, "jp: --reprovisiontpm only applies to create and update; attest never touches the TPM\n%s", usage)
		return exitUsage
	}

	if fi, err := os.Stat(pfile); err != nil || !fi.Mode().IsRegular() {
		fmt.Println("Provisioning file", pfile, "does not exist")
		return exitError
	}

	p, err := loadProvisioningFile(pfile)
	if err != nil {
		fmt.Println("Error processing", pfile+":", err)
		var verr *VarError
		if errors.As(err, &verr) {
			return exitUsage
		}
		return exitError
	}

	pv := NewProvisioner(p, op)
	pv.Recreate = recreate
	pv.ReprovisionTPM = reprovision
	if reprovision {
		fmt.Println("--reprovisiontpm: tpmclear and tpmprovision in the worklist will change the TPM")
	}
	key, err := apiKey(keyFile, os.Getenv, os.ReadFile)
	if err != nil {
		fmt.Println("jp:", err)
		return exitUsage
	}
	if key == "" && (os.Getenv("JANE_USERNAME") != "" || os.Getenv("JANE_PASSWORD") != "") {
		fmt.Println("jp: JANE_USERNAME and JANE_PASSWORD are no longer used (1.5.5): Jane's REST API uses API keys; set JANE_API_KEY or give --apikeyfile")
		return exitUsage
	}
	pv.Client.APIKey = key
	if key != "" && insecureRemote(p.AttestationServer) {
		fmt.Println("warning: sending the Jane API key over plain HTTP to", p.AttestationServer)
	}
	return pv.Run()
}

// apiKey picks the Jane REST API key: the contents of --apikeyfile when given,
// else JANE_API_KEY. An empty result means no key is sent. A key that is not
// "jane_" and 43 base64url characters is refused here, as Jane would refuse it.
func apiKey(file *string, getenv func(string) string, readFile func(string) ([]byte, error)) (string, error) {
	key := getenv("JANE_API_KEY")
	from := "JANE_API_KEY"
	if file != nil {
		b, err := readFile(*file)
		if err != nil {
			return "", fmt.Errorf("cannot read the API key file: %v", err)
		}
		key, from = string(b), *file
	}
	key = strings.TrimSpace(key)
	if key != "" && !validAPIKey(key) {
		return "", fmt.Errorf("%s does not hold a Jane API key (jane_ followed by 43 characters, as printed by addjaneuser --apikey)", from)
	}
	return key, nil
}

// validAPIKey mirrors Jane's check (auth/restkeys.go): "jane_" and 43 base64url characters.
func validAPIKey(k string) bool {
	const prefix = "jane_"
	if len(k) != len(prefix)+43 || !strings.HasPrefix(k, prefix) {
		return false
	}
	for _, ch := range k[len(prefix):] {
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

// insecureRemote reports whether the attestation server URL is plain HTTP to
// a host other than this machine.
func insecureRemote(server string) bool {
	u, err := url.Parse(server)
	if err != nil || u.Scheme != "http" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return false
	}
	return true
}

// Run performs the pre-flight checks on the element ID file and then the
// worklist, or the attestation, returning the process exit code.
func (pv *Provisioner) Run() int {
	// create and update write the element ID file (and use the TPM): stop
	// before anything reaches Jane if this user cannot (#46, packaging review).
	if pv.Op == "create" || pv.Op == "update" {
		if err := dirWritable(filepath.Dir(pv.IDFile)); err != nil {
			fmt.Printf("jp %s needs to write %s (and use the TPM): run it as root, e.g. sudo -E %s %s ...\n", pv.Op, pv.IDFile, os.Args[0], pv.Op)
			fmt.Println("Nothing was changed.")
			return exitError
		}
	}
	if err := pv.idFileUnreadable(); err != nil {
		fmt.Println("Error:", err)
		return exitError
	}
	exists := pv.idFileExists()

	if (pv.Op == "update" || pv.Op == "attest") && !exists {
		fmt.Println(pv.IDFile, "file is missing for an", pv.Op, "operation; run jp create first")
		return exitError
	}

	if pv.Op == "attest" {
		return pv.runAttest()
	}

	if pv.Op == "create" {
		if code, stop := pv.createPreflight(exists); stop {
			return code
		}
	}

	if _, err := pv.ProcessWorklist(); err != nil {
		fmt.Println("Error:", err)
		return exitError
	}

	fmt.Println("Complete.")
	return exitOK
}

func (pv *Provisioner) runAttest() int {
	report, err := pv.Attest()
	if report != nil {
		report.Print(pv)
	}
	if err != nil {
		fmt.Println("Error:", err)
		return exitError
	}
	if !report.Trusted() {
		return exitNotTrust
	}
	return exitOK
}
