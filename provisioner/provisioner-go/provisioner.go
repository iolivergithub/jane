package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-tpm/tpm2"

	"jp/tpm"
)

// Element is the element record sent to janeserver (a subset of its
// structures.Element). Optional sections are only sent when collected.
type Element struct {
	ItemID      string              `json:"itemid,omitempty"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Tags        []string            `json:"tags"`
	Endpoints   map[string]Endpoint `json:"endpoints"`
	Host        *HostInfo           `json:"host,omitempty"`
	UEFI        *UEFIInfo           `json:"uefi,omitempty"`
	TPM2        *TPM2               `json:"tpm2,omitempty"`
	IMA         *IMAInfo            `json:"ima,omitempty"`
}

type HostInfo struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Hostname  string `json:"hostname"`
	MachineID string `json:"machineid"`
}

type UEFIInfo struct {
	Eventlog string `json:"eventlog"`
}

type IMAInfo struct {
	ASCIILog string `json:"asciilog"`
}

const (
	uefiEventLog = "/sys/kernel/security/tpm0/binary_bios_measurements"
	imaASCIILog  = "/sys/kernel/security/ima/ascii_runtime_measurements"
)

// errQuit is returned when the operator answers "q" at a safety prompt.
var errQuit = errors.New("terminated by operator")

// Provisioner runs a provisioning file's worklist against a Jane server.
type Provisioner struct {
	P      *ProvisioningFile
	Client *Client
	Op     string // "create" or "update"

	// Unsafe skips the y/n/q safety prompts.
	Unsafe bool

	IDFile string // where the element's itemid is stored
	KeyDir string // where tpmprovision writes ek.pub, ak.pub and ak.name

	In       *bufio.Reader
	Out      io.Writer
	Now      func() time.Time
	HostInfo func() (HostInfo, error)

	// OpenTPM opens the TPM named by the provisioning file's tpm2.device.
	OpenTPM func(device string) (TPMDevice, string, error)
	tpmDev  TPMDevice
}

func NewProvisioner(p *ProvisioningFile, op string, unsafe bool) *Provisioner {
	return &Provisioner{
		P:        p,
		Client:   NewClient(p.AttestationServer),
		Op:       op,
		Unsafe:   unsafe,
		IDFile:   defaultIDFile(),
		KeyDir:   os.TempDir(),
		In:       bufio.NewReader(os.Stdin),
		Out:      os.Stdout,
		Now:      time.Now,
		HostInfo: collectHostInfo,
		OpenTPM:  openTPMDevice,
	}
}

// timeNow formats the current UTC time the way Python's
// str(datetime.now(timezone.utc)) does, e.g. 2026-09-29 19:53:12.123456+00:00.
func (pv *Provisioner) timeNow() string {
	return pv.Now().UTC().Format("2006-01-02 15:04:05.000000-07:00")
}

//
// Worklist
//

func (pv *Provisioner) initialElement() *Element {
	return &Element{
		Name:        pv.P.Element.Name,
		Description: pv.P.Element.Description,
		Tags:        pv.P.Element.Tags,
		Endpoints:   pv.P.Endpoints,
	}
}

// ProcessWorklist runs each worklist step in order and returns the element
// as built.
func (pv *Provisioner) ProcessWorklist() (*Element, error) {
	defer pv.closeTPM()
	e := pv.initialElement()
	eid := ""

	for _, w := range pv.P.ProvisionWorklist {
		switch w {
		case "tpmclear":
			ok, err := pv.ContinueQuestion("TPM Clear?")
			if err != nil {
				return e, err
			}
			if ok {
				if err := pv.tpmClear(); err != nil {
					return e, fmt.Errorf("tpmclear: %w", err)
				}
			}
		case "tpmprovision":
			ok, err := pv.ContinueQuestion("TPM Provision?")
			if err != nil {
				return e, err
			}
			if ok {
				if err := pv.tpmProvision(); err != nil {
					return e, fmt.Errorf("tpmprovision: %w", err)
				}
			}
		case "makebetterdescription":
			e.Description = pv.betterDescription()
		case "collecthostinfo":
			h, err := pv.HostInfo()
			if err != nil {
				return e, fmt.Errorf("collecthostinfo: %w", err)
			}
			e.Host = &h
		case "collectuefi":
			e.UEFI = &UEFIInfo{Eventlog: uefiEventLog}
		case "collecttpm2":
			e.TPM2 = pv.collectTPM2()
		case "collectima":
			e.IMA = &IMAInfo{ASCIILog: imaASCIILog}
		case "processelement":
			id, err := pv.processElement(e)
			if err != nil {
				return e, fmt.Errorf("processelement: %w", err)
			}
			eid = id
			if err := pv.writeID(eid); err != nil {
				return e, err
			}
		case "processevs", "processevs_withrules":
			if eid == "" {
				return e, fmt.Errorf("%s needs an element ID: put processelement before it in the worklist", w)
			}
			if err := pv.processEVs(eid, w == "processevs_withrules"); err != nil {
				return e, fmt.Errorf("%s: %w", w, err)
			}
		default:
			fmt.Fprintln(pv.Out, "Unknown provision work command", w)
		}
	}
	return e, nil
}

func (pv *Provisioner) betterDescription() string {
	return pv.P.Element.Description +
		"Element name: (" + pv.P.Element.Name + ") " +
		"Entry added at " + pv.timeNow() + " UTC"
}

func (pv *Provisioner) processElement(e *Element) (string, error) {
	switch pv.Op {
	case "create":
		fmt.Fprintln(pv.Out, "creating...")
		return pv.Client.CreateElement(e)
	case "update":
		id, err := pv.readID()
		if err != nil {
			return "", err
		}
		fmt.Fprintln(pv.Out, "updating element...", id)
		e.ItemID = id
		return pv.Client.UpdateElement(e)
	default:
		return "", fmt.Errorf("unknown operation %q", pv.Op)
	}
}

//
// Expected values and rules
//

func (pv *Provisioner) processEVs(eid string, runRules bool) (err error) {
	sid, err := pv.Client.OpenSession("Provisioning Session")
	if err != nil {
		return err
	}
	defer func() {
		if cerr := pv.Client.CloseSession(sid); err == nil {
			err = cerr
		}
	}()

	for _, ev := range pv.P.EVS {
		pid, epn := ev.IntentID, ev.Spec.Protocol
		fmt.Fprintln(pv.Out, eid, pid, epn, sid)

		cid, err := pv.Client.Attest(eid, pid, epn, sid)
		if err != nil {
			return err
		}
		claim, err := pv.Client.GetClaim(cid)
		if err != nil {
			return err
		}

		if ev.Spec.Type != "" {
			if err := pv.processExpectedValueType(ev, eid, claim); err != nil {
				return fmt.Errorf("intent %s: %w", pid, err)
			}
		}

		if runRules {
			for _, r := range ev.Spec.Rules {
				v, err := pv.Client.Verify(cid, sid, r)
				if err != nil {
					return fmt.Errorf("rule %s: %w", r, err)
				}
				fmt.Fprintln(pv.Out, "  ... ... ", r, "...", v.Describe())
			}
		}
	}
	return nil
}

// Claim values are kept as raw JSON so they reach the expected value
// unchanged (a uint64 firmwareVersion would lose precision as float64).
type machineIDBody struct {
	MachineID json.RawMessage `json:"machineid"`
}

type tpm2QuoteBody struct {
	Quote struct {
		Attested struct {
			PCRDigest json.RawMessage `json:"pcrdigest"`
		} `json:"attested"`
		FirmwareVersion json.RawMessage `json:"firmwareVersion"`
	} `json:"quote"`
}

func missing(v json.RawMessage) bool {
	return len(v) == 0 || string(v) == "null"
}

// processExpectedValueType turns a claim into an expected value for the
// intent, according to the evs entry's type.
func (pv *Provisioner) processExpectedValueType(ev EVEntry, eid string, c *Claim) error {
	pid, epn, typ := ev.IntentID, ev.Spec.Protocol, ev.Spec.Type
	name := pv.P.Element.Name + "---" + pid
	desc := pv.P.Element.Name + "---" + typ + " at " + pv.timeNow() + " UTC"

	var evs map[string]any
	switch typ {
	case "sysmachineid":
		var b machineIDBody
		if err := json.Unmarshal(c.Body, &b); err != nil {
			return err
		}
		if missing(b.MachineID) {
			return errors.New("claim body has no machineid")
		}
		evs = map[string]any{"machineid": b.MachineID}

	case "tpm2quote":
		var b tpm2QuoteBody
		if err := json.Unmarshal(c.Body, &b); err != nil {
			return err
		}
		if missing(b.Quote.Attested.PCRDigest) || missing(b.Quote.FirmwareVersion) {
			return errors.New("claim body has no quote pcrdigest/firmwareVersion")
		}
		fmt.Fprintln(pv.Out, "*********************")
		fmt.Fprintln(pv.Out, string(b.Quote.Attested.PCRDigest))
		fmt.Fprintln(pv.Out, string(b.Quote.FirmwareVersion))
		fmt.Fprintln(pv.Out, "*********************")
		// The original appends the intent ID a second time for quotes; kept
		// so names match EVs already created by the Python provisioner.
		name = name + "-" + pid
		evs = map[string]any{
			"attestedValue":   b.Quote.Attested.PCRDigest,
			"firmwareVersion": b.Quote.FirmwareVersion,
		}

	default:
		fmt.Fprintln(pv.Out, "Unknown expected value type", typ, "for", pid)
		return nil
	}

	return pv.createOrUpdateEV(&ExpectedValue{
		Name: name, Description: desc, EVS: evs,
		ElementID: eid, IntentID: pid, EndpointName: epn,
	})
}

func (pv *Provisioner) createOrUpdateEV(ev *ExpectedValue) error {
	id, found, err := pv.Client.FindExpectedValue(ev.ElementID, ev.IntentID, ev.EndpointName)
	if err != nil {
		return err
	}
	if found {
		fmt.Fprintln(pv.Out, " ... updating evs...", id)
		ev.ItemID = id
		return pv.Client.UpdateExpectedValue(ev)
	}
	fmt.Fprintln(pv.Out, " ... creating evs")
	return pv.Client.CreateExpectedValue(ev)
}

//
// TPM (native, via go-tpm)
//

// TPMDevice is the set of TPM operations the worklist uses; *tpm.TPM
// provides them over a real transport.
type TPMDevice interface {
	Evict(h tpm2.TPMHandle) (bool, error)
	CreateEK(h tpm2.TPMHandle) (*tpm.Key, error)
	CreateAK(ek, ak tpm2.TPMHandle) (*tpm.Key, error)
	ReadKey(h tpm2.TPMHandle) (*tpm.Key, error)
	PersistentHandles() ([]tpm2.TPMHandle, error)
	Close() error
}

type tpmConn struct {
	*tpm.TPM
	io.Closer
}

func openTPMDevice(device string) (TPMDevice, string, error) {
	t, desc, err := tpm.Open(device)
	if err != nil {
		return nil, "", err
	}
	return tpmConn{&tpm.TPM{T: t}, t}, desc, nil
}

// device opens the TPM on first use and keeps it open for the run.
func (pv *Provisioner) device() (TPMDevice, error) {
	if pv.tpmDev != nil {
		return pv.tpmDev, nil
	}
	d, desc, err := pv.OpenTPM(pv.P.TPM2.Device)
	if err != nil {
		return nil, fmt.Errorf("opening TPM %q: %w", pv.P.TPM2.Device, err)
	}
	fmt.Fprintln(pv.Out, "Using", desc)
	pv.tpmDev = d
	return d, nil
}

func (pv *Provisioner) closeTPM() {
	if pv.tpmDev != nil {
		pv.tpmDev.Close()
		pv.tpmDev = nil
	}
}

func (pv *Provisioner) handles() (ek, ak tpm2.TPMHandle, err error) {
	if ek, err = tpm.ParseHandle(pv.P.TPM2.EK.Handle); err != nil {
		return 0, 0, fmt.Errorf("tpm2.ek.handle: %w", err)
	}
	if ak, err = tpm.ParseHandle(pv.P.TPM2.AK.Handle); err != nil {
		return 0, 0, fmt.Errorf("tpm2.ak.handle: %w", err)
	}
	return ek, ak, nil
}

// tpmClear evicts the persistent EK and AK. A handle with nothing at it is
// reported, not an error.
func (pv *Provisioner) tpmClear() error {
	ek, ak, err := pv.handles()
	if err != nil {
		return err
	}
	d, err := pv.device()
	if err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "ek,ak", tpm.Hex(ek), tpm.Hex(ak))
	for _, h := range []tpm2.TPMHandle{ek, ak} {
		existed, err := d.Evict(h)
		if err != nil {
			return err
		}
		if existed {
			fmt.Fprintln(pv.Out, "  evicted", tpm.Hex(h))
		} else {
			fmt.Fprintln(pv.Out, "  nothing at", tpm.Hex(h))
		}
	}
	return nil
}

// tpmProvision creates and persists the EK and AK, writes their public parts
// to KeyDir as tpm2_createek/tpm2_createak did (ek.pub as TPM2B_PUBLIC,
// ak.pub as PEM, ak.name as the raw name), and lists persistent handles.
func (pv *Provisioner) tpmProvision() error {
	ekH, akH, err := pv.handles()
	if err != nil {
		return err
	}
	d, err := pv.device()
	if err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "ek,ak", tpm.Hex(ekH), tpm.Hex(akH))

	ek, err := d.CreateEK(ekH)
	if err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "  created EK at", tpm.Hex(ekH), "name", ek.NameHex())
	if err := pv.writeKeyFile("ek.pub", tpm2.Marshal(&ek.Public)); err != nil {
		return err
	}

	ak, err := d.CreateAK(ekH, akH)
	if err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "  created AK at", tpm.Hex(akH), "name", ak.NameHex())
	akPEM, err := ak.PublicPEM()
	if err != nil {
		return err
	}
	if err := pv.writeKeyFile("ak.pub", akPEM); err != nil {
		return err
	}
	if err := pv.writeKeyFile("ak.name", ak.Name.Buffer); err != nil {
		return err
	}

	hs, err := d.PersistentHandles()
	if err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "  persistent handles:")
	for _, h := range hs {
		fmt.Fprintln(pv.Out, "  -", tpm.Hex(h))
	}
	return nil
}

func (pv *Provisioner) writeKeyFile(name string, data []byte) error {
	p := filepath.Join(pv.KeyDir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintln(pv.Out, "  wrote", p)
	return nil
}

// collectTPM2 copies the provisioning file's tpm2 section and fills in the
// EK and AK public areas and names from the TPM where the file leaves them
// empty. The TPM is optional here: if it or a key is unavailable, the
// section is sent as written, with a note.
func (pv *Provisioner) collectTPM2() *TPM2 {
	t := pv.P.TPM2
	d, err := pv.device()
	if err != nil {
		fmt.Fprintln(pv.Out, "  note: tpm2 keys not read:", err)
		return &t
	}
	for _, k := range []*TPMKey{&t.EK, &t.AK} {
		if k.Public != "" && k.Name != "" {
			continue
		}
		h, err := tpm.ParseHandle(k.Handle)
		if err != nil {
			fmt.Fprintln(pv.Out, "  note:", err)
			continue
		}
		key, err := d.ReadKey(h)
		if err != nil {
			fmt.Fprintln(pv.Out, "  note:", err)
			continue
		}
		if key == nil {
			fmt.Fprintln(pv.Out, "  note: no key at", tpm.Hex(h))
			continue
		}
		if k.Public == "" {
			k.Public = key.PublicBase64()
		}
		if k.Name == "" {
			k.Name = key.NameHex()
		}
	}
	return &t
}

//
// Element ID file
//

func (pv *Provisioner) writeID(id string) error {
	if err := os.MkdirAll(filepath.Dir(pv.IDFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pv.IDFile, []byte(id), 0o644)
}

func (pv *Provisioner) readID() (string, error) {
	b, err := os.ReadFile(pv.IDFile)
	if err != nil {
		return "", err
	}
	id, _, _ := strings.Cut(string(b), "\n")
	id = strings.TrimRight(id, "\r")
	if id == "" {
		return "", fmt.Errorf("%s is empty", pv.IDFile)
	}
	return id, nil
}

func (pv *Provisioner) idFileExists() bool {
	fi, err := os.Stat(pv.IDFile)
	return err == nil && fi.Mode().IsRegular()
}

//
// Safety prompt
//

// ContinueQuestion asks the operator to confirm a step. It returns true
// straight away in unsafe mode, and errQuit if the operator answers q.
func (pv *Provisioner) ContinueQuestion(msg string) (bool, error) {
	if pv.Unsafe {
		return true, nil
	}
	for {
		fmt.Fprintf(pv.Out, "Safety check: %s (ynq) >", msg)
		line, err := pv.In.ReadString('\n')
		switch strings.TrimSpace(line) {
		case "y":
			return true, nil
		case "n":
			return false, nil
		case "q":
			return false, errQuit
		}
		if err != nil {
			return false, fmt.Errorf("reading answer: %w", err)
		}
		fmt.Fprintln(pv.Out, "valid reponses are y=yes, n=no and q=quit now")
	}
}
