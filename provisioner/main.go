// Command jp is the Jane element provisioner: it registers this machine as
// an element with a Jane attestation server and, optionally, provisions the
// TPM and records expected values; it can then attest the element.
//
//	jp create|update|attest <provisioning file> [-u|--unsafe] [--recreate]
package main

import (
	"errors"
	"fmt"
	"os"
)

const usage = `usage: jp [-h] [-u] [--recreate] operation pfile

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
  -u, --unsafe   do not ask for confirmation before TPM and overwrite steps
  --recreate     create only: register a new element even though this
                 machine is already registered (asks first unless -u);
                 the old element is left in Jane, orphaned

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
	unsafe, recreate := false, false
	var pos []string
	for _, a := range argv {
		switch a {
		case "-u", "--unsafe":
			unsafe = true
		case "--recreate":
			recreate = true
		case "-h", "--help":
			fmt.Print(usage)
			return exitOK
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

	if unsafe {
		fmt.Println("unsafe mode: safety prompts are skipped")
	}

	if op != "create" && op != "update" && op != "attest" {
		fmt.Println("Unknown command, not one of: create, update, attest")
		return exitUsage
	}

	if recreate && op != "create" {
		fmt.Fprintf(os.Stderr, "jp: --recreate only applies to create\n%s", usage)
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

	pv := NewProvisioner(p, op, unsafe)
	pv.Recreate = recreate
	return pv.Run()
}

// Run performs the pre-flight checks on the element ID file and then the
// worklist, or the attestation, returning the process exit code.
func (pv *Provisioner) Run() int {
	if pv.Op == "update" || pv.Op == "attest" {
		if err := pv.migrateIDFile(); err != nil {
			fmt.Println("Error:", err)
			return exitError
		}
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
		if errors.Is(err, errQuit) {
			fmt.Println("Terminating immediately")
			return exitOK
		}
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
