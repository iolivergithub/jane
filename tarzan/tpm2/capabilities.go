package tpm2

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// Capabilities runs TPM2_GetCapability and reports the TPM's identity and
// configuration: fixed and variable properties, allocated PCR banks,
// algorithms, ECC curves, persistent handles (#69) and the commands the TPM
// implements (#103). Nothing is signed, so
// this describes the TPM rather than proves anything about it.
//
// The algorithm, ECC curve and command queries are optional (#124): if the
// TPM refuses one, its section is left empty and the reason is reported under
// "unavailable"; the rest of the claim is still returned. On a TPM without
// ECC the curves are not queried at all (some, e.g. an Infineon SLB 9665 with
// firmware 5.0, answer that query with TPM_RC_VALUE instead of an empty list).
//
// Parameters:
//
//	tpm2/device  TPM device, default /dev/tpm0
func Capabilities(c echo.Context) error {

	fmt.Println("tpm2 capabilities called")

	ps := new(map[string]interface{})
	if err := c.Bind(&ps); err != nil {
		fmt.Printf("Capabilities BIND err %v\n", err.Error())
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not decode parameters %v", err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	tpm2device := tpmDeviceParam(*ps)

	tpm, err := OpenTPM(tpm2device)
	if err != nil {
		rtn := tpm2taErrorReturn{fmt.Sprintf("Could not open specified TPM %v during Capabilities function with error %v", tpm2device, err.Error())}
		return c.JSON(http.StatusUnprocessableEntity, rtn)
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			fmt.Printf("\ncan't close TPM %q: %v", tpm2device, err)
		}
	}()

	rtn, err := capabilities(tpm)
	if err != nil {
		fmt.Printf("Could not read the TPM capabilities with error %v\n", err.Error())
		return c.JSON(http.StatusUnprocessableEntity, tpm2taErrorReturn{err.Error()})
	}
	return c.JSON(http.StatusOK, rtn)
}

// tpm2capabilitiesReturn is the result of tpm2/capabilities. Properties are
// keyed by their TCG names (TPM_PT_...), or by hex ID if unknown.
type tpm2capabilitiesReturn struct {
	Summary           capabilitiesSummary `json:"summary"`
	Properties        map[string]string   `json:"properties"`
	PCRBanks          map[string]int      `json:"pcrbanks"` // bank -> number of PCRs allocated
	Algorithms        []string            `json:"algorithms"`
	ECCCurves         []string            `json:"ecccurves"`
	PersistentHandles []string            `json:"persistenthandles"`
	Commands          []string            `json:"commands"` // e.g. "TPM_CC_Quote (0x158)"

	// Unavailable gives, for an optional section (algorithms, ecccurves,
	// commands) that could not be read, the reason (#124).
	Unavailable map[string]string `json:"unavailable,omitempty"`
}

// eccNotSupported is the reason given when the ECC curves are not queried.
const eccNotSupported = "ECC not supported by this TPM (TPM_ALG_ECC is not in its algorithm list)"

// unavailable records why an optional section could not be read.
func (r *tpm2capabilitiesReturn) unavailable(section, reason string) {
	if r.Unavailable == nil {
		r.Unavailable = map[string]string{}
	}
	r.Unavailable[section] = reason
}

// hasAlgorithm reports whether a is in the TPM's algorithm list.
func hasAlgorithm(algs []tpm2.TPMAlgID, a tpm2.TPMAlgID) bool {
	for _, x := range algs {
		if x == a {
			return true
		}
	}
	return false
}

// capabilitiesSummary picks out the properties that identify the TPM.
type capabilitiesSummary struct {
	Manufacturer    string `json:"manufacturer"`
	VendorString    string `json:"vendorstring"`
	FirmwareVersion string `json:"firmwareversion"`
	Family          string `json:"family"`
	Level           string `json:"level"`
	Revision        string `json:"revision"`
	FIPS1402        bool   `json:"fips140_2"`
}

// capabilityMax is how many items are asked for at a time; the TPM may
// return fewer and say there are more.
const capabilityMax = 64

// getCapability calls TPM2_GetCapability from property onwards until the TPM
// has no more data, passing each response's data to add, which returns the
// next property to ask for.
func getCapability(tpm transport.TPM, capability tpm2.TPMCap, property uint32, add func(tpm2.TPMUCapabilities) (uint32, error)) error {
	for {
		rsp, err := tpm2.GetCapability{Capability: capability, Property: property, PropertyCount: capabilityMax}.Execute(tpm)
		if err != nil {
			return fmt.Errorf("TPM2_GetCapability(%v, 0x%x) failed: %v", capability, property, err)
		}
		next, err := add(rsp.CapabilityData.Data)
		if err != nil {
			return err
		}
		if !rsp.MoreData || next <= property {
			return nil
		}
		property = next
	}
}

func capabilities(tpm transport.TPM) (tpm2capabilitiesReturn, error) {
	rtn := tpm2capabilitiesReturn{Properties: map[string]string{}, PCRBanks: map[string]int{}}
	raw := map[tpm2.TPMPT]uint32{}

	// fixed (TPM_PT_FIXED, 0x100...) and variable (TPM_PT_VAR, 0x200...) properties
	err := getCapability(tpm, tpm2.TPMCapTPMProperties, uint32(tpm2.TPMPTFamilyIndicator), func(d tpm2.TPMUCapabilities) (uint32, error) {
		props, err := d.TPMProperties()
		if err != nil {
			return 0, err
		}
		next := uint32(0)
		for _, p := range props.TPMProperty {
			raw[p.Property] = p.Value
			rtn.Properties[propertyName(p.Property)] = propertyValue(p.Property, p.Value)
			next = uint32(p.Property) + 1
		}
		return next, nil
	})
	if err != nil {
		return rtn, err
	}

	rtn.Summary = capabilitiesSummary{
		Manufacturer:    fourChars(raw[tpm2.TPMPTManufacturer]),
		VendorString:    strings.TrimSpace(fourChars(raw[tpm2.TPMPTVendorString1]) + fourChars(raw[tpm2.TPMPTVendorString2]) + fourChars(raw[tpm2.TPMPTVendorString3]) + fourChars(raw[tpm2.TPMPTVendorString4])),
		FirmwareVersion: fmt.Sprintf("%08x%08x", raw[tpm2.TPMPTFirmwareVersion1], raw[tpm2.TPMPTFirmwareVersion2]),
		Family:          fourChars(raw[tpm2.TPMPTFamilyIndicator]),
		Level:           fmt.Sprintf("%d", raw[tpm2.TPMPTLevel]),
		Revision:        fmt.Sprintf("%d.%02d", raw[tpm2.TPMPTRevision]/100, raw[tpm2.TPMPTRevision]%100),
		FIPS1402:        raw[tpm2.TPMPTModes]&1 == 1,
	}

	// allocated PCR banks
	err = getCapability(tpm, tpm2.TPMCapPCRs, 0, func(d tpm2.TPMUCapabilities) (uint32, error) {
		sels, err := d.AssignedPCR()
		if err != nil {
			return 0, err
		}
		for _, s := range sels.PCRSelections {
			n := 0
			for _, b := range s.PCRSelect {
				for ; b != 0; b &= b - 1 {
					n++
				}
			}
			name, ok := bankNames[tpm2.TPMAlgID(s.Hash)]
			if !ok {
				name = algName(tpm2.TPMAlgID(s.Hash))
			}
			rtn.PCRBanks[name] = n
		}
		return 0, nil
	})
	if err != nil {
		return rtn, err
	}

	// algorithms, from TPM_ALG_FIRST (0x0001): some TPMs reject 0 (#105);
	// optional (#124)
	var algIDs []tpm2.TPMAlgID
	err = getCapability(tpm, tpm2.TPMCapAlgs, 0x0001, func(d tpm2.TPMUCapabilities) (uint32, error) {
		algs, err := d.Algorithms()
		if err != nil {
			return 0, err
		}
		next := uint32(0)
		for _, a := range algs.AlgProperties {
			algIDs = append(algIDs, a.Alg)
			rtn.Algorithms = append(rtn.Algorithms, algName(a.Alg))
			next = uint32(a.Alg) + 1
		}
		return next, nil
	})
	algsKnown := err == nil
	if err != nil {
		rtn.Algorithms = nil
		rtn.unavailable("algorithms", err.Error())
	}

	// ECC curves, from TPM_ECC_NIST_P192 (0x0001): Intel PTT rejects
	// TPM_ECC_NONE (0) with TPM_RC_VALUE (#105). Not asked of a TPM without
	// ECC, which may refuse the query (#124); asked if the algorithm list
	// could not be read. Optional.
	if algsKnown && !hasAlgorithm(algIDs, tpm2.TPMAlgECC) {
		rtn.ECCCurves = []string{}
		rtn.unavailable("ecccurves", eccNotSupported)
	} else {
		err = getCapability(tpm, tpm2.TPMCapECCCurves, 0x0001, func(d tpm2.TPMUCapabilities) (uint32, error) {
			curves, err := d.ECCCurves()
			if err != nil {
				return 0, err
			}
			next := uint32(0)
			for _, c := range curves.ECCCurves {
				rtn.ECCCurves = append(rtn.ECCCurves, curveName(c))
				next = uint32(c) + 1
			}
			return next, nil
		})
		if err != nil {
			rtn.ECCCurves = nil
			rtn.unavailable("ecccurves", err.Error())
		}
	}

	// implemented commands, from TPM_CC_FIRST (0x011F) (#103); optional (#124)
	err = getCapability(tpm, tpm2.TPMCapCommands, 0x011F, func(d tpm2.TPMUCapabilities) (uint32, error) {
		cmds, err := d.Command()
		if err != nil {
			return 0, err
		}
		next := uint32(0)
		for _, a := range cmds.CommandAttributes {
			code := commandCode(a)
			rtn.Commands = append(rtn.Commands, commandName(code))
			next = code + 1
		}
		return next, nil
	})
	if err != nil {
		rtn.Commands = nil
		rtn.unavailable("commands", err.Error())
	}

	// persistent handles (0x81xxxxxx)
	err = getCapability(tpm, tpm2.TPMCapHandles, 0x81000000, func(d tpm2.TPMUCapabilities) (uint32, error) {
		handles, err := d.Handles()
		if err != nil {
			return 0, err
		}
		next := uint32(0)
		for _, h := range handles.Handle {
			if uint32(h)>>24 != 0x81 {
				return 0, nil // past the persistent range
			}
			rtn.PersistentHandles = append(rtn.PersistentHandles, handleHex(h))
			next = uint32(h) + 1
		}
		return next, nil
	})
	return rtn, err
}

// propertyName is a property's TCG name, or its ID in hex if unknown.
func propertyName(p tpm2.TPMPT) string {
	if n, ok := tpmPropertyNames[uint32(p)]; ok {
		return n
	}
	return fmt.Sprintf("0x%08x", uint32(p))
}

// commandCode is the command code (TPM_CC) of a command's attributes: the
// index, with bit 29 set for a vendor-specific command.
func commandCode(a tpm2.TPMACC) uint32 {
	code := uint32(a.CommandIndex)
	if a.V {
		code |= 0x20000000
	}
	return code
}

// commandName is a command's TCG name and code, e.g. "TPM_CC_Quote (0x158)",
// or just the code in hex if unknown (newer commands, vendor commands).
func commandName(code uint32) string {
	if n, ok := tpmCommandNames[code]; ok {
		return fmt.Sprintf("%s (0x%x)", n, code)
	}
	if code&0x20000000 != 0 {
		return fmt.Sprintf("vendor command (0x%08x)", code)
	}
	return fmt.Sprintf("0x%x", code)
}

// propertyValue decodes the properties that have a text form; the rest are
// given in decimal, and bit fields in hex.
func propertyValue(p tpm2.TPMPT, v uint32) string {
	switch p {
	case tpm2.TPMPTFamilyIndicator, tpm2.TPMPTManufacturer, tpm2.TPMPTPSFamilyIndicator,
		tpm2.TPMPTVendorString1, tpm2.TPMPTVendorString2, tpm2.TPMPTVendorString3, tpm2.TPMPTVendorString4:
		return fourChars(v)
	case tpm2.TPMPTFirmwareVersion1, tpm2.TPMPTFirmwareVersion2, tpm2.TPMPTModes,
		tpm2.TPMPTPermanent, tpm2.TPMPTStartupClear, tpm2.TPMPTMemory:
		return fmt.Sprintf("0x%08x", v)
	}
	return fmt.Sprintf("%d", v)
}

// fourChars is a property holding up to four ASCII characters, e.g. the
// manufacturer ID "IFX", without trailing NULs or spaces.
func fourChars(v uint32) string {
	b := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	var out []byte
	for _, c := range b {
		if c >= 0x20 && c <= 0x7e {
			out = append(out, c)
		}
	}
	return strings.TrimSpace(string(out))
}

// algName is an algorithm's TCG name, e.g. TPM_ALG_SHA256, or its ID in
// hex if unknown.
func algName(a tpm2.TPMAlgID) string {
	if n, ok := tpmAlgorithmNames[uint16(a)]; ok {
		return n
	}
	return fmt.Sprintf("0x%04x", uint16(a))
}

// curveName is an ECC curve's TCG name, e.g. TPM_ECC_NIST_P256, or its ID in
// hex if unknown.
func curveName(c tpm2.TPMECCCurve) string {
	if n, ok := tpmECCCurveNames[uint16(c)]; ok {
		return n
	}
	return fmt.Sprintf("0x%04x", uint16(c))
}
