package main

import (
	"errors"
	"fmt"
	"strings"
)

// registration is an element in Jane that looks like this machine.
type registration struct {
	ItemID string
	Reason string // why it matched, e.g. `same name "Debian13VM"`
}

// createPreflight runs before create's worklist, so before any TPM step. It
// refuses to create when the machine is already registered: the element ID
// file exists, or Jane holds an element with the same name or machine ID.
// Unsafe does not lift the refusal; only Recreate does, and that still asks
// unless Unsafe is set. It returns stop=true with the exit code when create
// must not go ahead.
func (pv *Provisioner) createPreflight(idFileExists bool) (code int, stop bool) {
	var reasons []string
	if idFileExists {
		id, _ := pv.readID()
		reasons = append(reasons, fmt.Sprintf("element ID file %s exists (element %s)", pv.existingIDFile(), orUnknown(id)))
	}

	regs, err := pv.findRegistrations()
	if err != nil {
		fmt.Fprintln(pv.Out, "Could not check Jane for an existing element:", err)
		fmt.Fprintln(pv.Out, "Nothing was changed.")
		return exitError, true
	}
	for _, r := range regs {
		reasons = append(reasons, fmt.Sprintf("Jane has element %s with the %s", r.ItemID, r.Reason))
	}

	if len(reasons) == 0 {
		return 0, false
	}

	fmt.Fprintln(pv.Out, "######################################################")
	fmt.Fprintln(pv.Out, "This machine looks already registered:")
	for _, r := range reasons {
		fmt.Fprintln(pv.Out, "  -", r)
	}

	if !pv.Recreate {
		fmt.Fprintln(pv.Out, "Refusing to create: run 'jp update' to refresh the existing element and its")
		fmt.Fprintln(pv.Out, "expected values, or 'jp create --recreate' to register a new element anyway")
		fmt.Fprintln(pv.Out, "(the old one is left in Jane, and a tpmclear/tpmprovision in the worklist")
		fmt.Fprintln(pv.Out, "will invalidate its keys). Nothing was changed.")
		return exitError, true
	}

	fmt.Fprintln(pv.Out, "--recreate: a new element will be registered; the old one is left in Jane,")
	fmt.Fprintln(pv.Out, "and its TPM keys will no longer match if the worklist re-provisions the TPM.")
	ok, err := pv.ContinueQuestion("Recreate this element?")
	if errors.Is(err, errQuit) || (err == nil && !ok) {
		fmt.Fprintln(pv.Out, "Terminating safely.")
		return exitOK, true
	}
	if err != nil {
		fmt.Fprintln(pv.Out, "Error:", err)
		return exitError, true
	}
	return 0, false
}

// findRegistrations asks Jane for elements with this provisioning file's
// element name, or with this machine's machine ID. Any failure to ask is an
// error: create does not go ahead on a guess.
func (pv *Provisioner) findRegistrations() ([]registration, error) {
	var regs []registration
	seen := map[string]int{} // itemid -> index in regs

	add := func(id, reason string) {
		if i, ok := seen[id]; ok {
			regs[i].Reason += " and " + reason
			return
		}
		seen[id] = len(regs)
		regs = append(regs, registration{id, reason})
	}

	if name := pv.P.Element.Name; name != "" {
		ids, err := pv.Client.ElementIDsByName(name)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			add(id, fmt.Sprintf("same name %q", name))
		}
	}

	host, err := pv.HostInfo()
	if err != nil || host.MachineID == "" {
		fmt.Fprintln(pv.Out, "Warning: no machine ID here, so Jane was only checked by element name")
		return regs, nil
	}
	ids, err := pv.Client.ElementIDs()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		e, err := pv.Client.GetElement(id)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(e.Host.MachineID, host.MachineID) {
			add(id, fmt.Sprintf("same machine ID %s", host.MachineID))
		}
	}
	return regs, nil
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
