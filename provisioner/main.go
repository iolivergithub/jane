// Command jp is the Jane element provisioner: it registers this machine as
// an element with a Jane attestation server and, optionally, provisions the
// TPM and records expected values.
//
//	jp create|update <provisioning file> [-u|--unsafe]
package main

import (
	"errors"
	"fmt"
	"os"
)

const usage = `usage: jp [-h] [-u] operation pfile

Jane Element Configurator

positional arguments:
  operation      create or update
  pfile          provisioning file (YAML)

options:
  -h, --help     show this help message and exit
  -u, --unsafe   do not ask for confirmation before TPM and overwrite steps
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fmt.Println("Jane Element Configuration")

	// Flags may appear anywhere, as with the original argparse CLI.
	unsafe := false
	var pos []string
	for _, a := range argv {
		switch a {
		case "-u", "--unsafe":
			unsafe = true
		case "-h", "--help":
			fmt.Print(usage)
			return 0
		default:
			if len(a) > 1 && a[0] == '-' {
				fmt.Fprintf(os.Stderr, "jp: unknown option %s\n%s", a, usage)
				return 2
			}
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	op, pfile := pos[0], pos[1]

	if unsafe {
		fmt.Println("unsafe mode: safety prompts are skipped")
	}

	if op != "create" && op != "update" {
		fmt.Println("Unknown command, not one of: create, update")
		return 2
	}

	if fi, err := os.Stat(pfile); err != nil || !fi.Mode().IsRegular() {
		fmt.Println("Provisioning file", pfile, "does not exist")
		return 1
	}

	p, err := loadProvisioningFile(pfile)
	if err != nil {
		fmt.Println("Error processing", pfile+":", err)
		return 1
	}

	pv := NewProvisioner(p, op, unsafe)
	return pv.Run()
}

// Run performs the pre-flight checks on the element ID file and then the
// worklist, returning the process exit code.
func (pv *Provisioner) Run() int {
	exists := pv.idFileExists()

	if pv.Op == "update" && !exists {
		fmt.Println(pv.IDFile, "file is missing for an update operation")
		return 1
	}

	if pv.Op == "create" && exists {
		fmt.Println("######################################################")
		ok, err := pv.ContinueQuestion("Create mode and element ID file exists?")
		if errors.Is(err, errQuit) || (err == nil && !ok) {
			fmt.Println("Terminating safely.")
			return 0
		}
		if err != nil {
			fmt.Println("Error:", err)
			return 1
		}
	}

	if _, err := pv.ProcessWorklist(); err != nil {
		if errors.Is(err, errQuit) {
			fmt.Println("Terminating immediately")
			return 0
		}
		fmt.Println("Error:", err)
		return 1
	}

	fmt.Println("Complete.")
	return 0
}
