package webui

// Deeper decoding of UEFI event log events for the claim view (#135):
// display only, no rules. Each decoder returns detail lines shown under the
// event's description; nothing here is trusted or verified.

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/google/go-eventlog/tcg"
)

// maxDetailLines bounds the detail lines of one event (e.g. a dbx with
// hundreds of hashes).
const maxDetailLines = 40

// eventDetails decodes an event's data more deeply, by type.
func eventDetails(typ string, pcr int, data []byte) []string {
	var lines []string
	switch typ {
	case "EV_EFI_VARIABLE_DRIVER_CONFIG", "EV_EFI_VARIABLE_BOOT", "EV_EFI_VARIABLE_BOOT2":
		lines = variableDetails(data)
	case "EV_EFI_VARIABLE_AUTHORITY":
		lines = authorityDetails(data)
	case "EV_EFI_PLATFORM_FIRMWARE_BLOB", "EV_POST_CODE":
		if len(data) == 16 {
			lines = []string{fmt.Sprintf("Firmware blob at 0x%x, %d bytes", binary.LittleEndian.Uint64(data), binary.LittleEndian.Uint64(data[8:]))}
		}
	case "EV_EFI_PLATFORM_FIRMWARE_BLOB2":
		if len(data) >= 1 && len(data) >= 1+int(data[0])+16 {
			n := int(data[0])
			d := data[1+n:]
			lines = []string{fmt.Sprintf("%s at 0x%x, %d bytes", strings.TrimRight(string(data[1:1+n]), "\x00"),
				binary.LittleEndian.Uint64(d), binary.LittleEndian.Uint64(d[8:]))}
		}
	case "EV_EFI_BOOT_SERVICES_APPLICATION", "EV_EFI_BOOT_SERVICES_DRIVER", "EV_EFI_RUNTIME_SERVICES_DRIVER":
		if img, err := tcg.ParseEFIImageLoad(bytes.NewReader(data)); err == nil {
			if elems, err := img.DevicePath(); err == nil {
				lines = []string{"Device path: " + devicePathText(elems), fmt.Sprintf("Image size: %d bytes", img.Header.Length)}
			}
		}
	case "EV_EFI_GPT_EVENT":
		lines = gptDetails(data)
	case "EV_NO_ACTION":
		lines = noActionDetails(data)
	case "EV_EFI_HANDOFF_TABLES", "EV_EFI_HANDOFF_TABLES2":
		lines = handoffDetails(typ, data)
	case "EV_IPL":
		switch pcr {
		case 8:
			lines = []string{"Bootloader command or command line (PCR 8)"}
		case 9:
			lines = []string{"File read by the bootloader (PCR 9)"}
		}
	}
	if len(lines) > maxDetailLines {
		lines = append(lines[:maxDetailLines], fmt.Sprintf("… and %d more", len(lines)-maxDetailLines))
	}
	return lines
}

// variableDetails: the value of SecureBoot and boot variables, and the
// certificates and hashes in signature databases (PK, KEK, db, dbx, ...).
func variableDetails(data []byte) []string {
	v, err := tcg.ParseUEFIVariableData(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	name, guid := v.VarName(), v.Header.VariableName.String()
	if s := decodeKnownEfiVar(name, guid, v.VariableData); s != "" {
		lines := []string{name + ": " + s}
		if isLoadOptionName(name) {
			if dp := loadOptionDevicePath(v.VariableData); dp != "" {
				lines = append(lines, "Device path: "+dp)
			}
		}
		return lines
	}
	if len(v.VariableData) == 0 {
		return []string{name + ": empty"}
	}
	certs, hashes, err := v.SignatureData()
	if err != nil || (len(certs) == 0 && len(hashes) == 0) {
		return nil
	}
	lines := []string{fmt.Sprintf("%s: %d certificates, %d hashes", name, len(certs), len(hashes))}
	for _, c := range certs {
		lines = append(lines, certificateLine(&c))
	}
	for _, h := range hashes {
		lines = append(lines, "hash "+hex.EncodeToString(h))
	}
	return lines
}

// authorityDetails: the certificate that verified an image, or shim's SBAT
// level (text).
func authorityDetails(data []byte) []string {
	v, err := tcg.ParseUEFIVariableData(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	if s, ok := printableText(v.VariableData); ok {
		return []string{v.VarName() + ": " + s}
	}
	a, err := tcg.ParseUEFIVariableAuthority(v)
	if err != nil && len(a.Certs) == 0 {
		return []string{v.VarName() + ": could not be decoded: " + err.Error()}
	}
	var lines []string
	for _, c := range a.Certs {
		lines = append(lines, "Verified by "+certificateLine(&c))
	}
	return lines
}

func certificateLine(c *x509.Certificate) string {
	h := sha256.Sum256(c.Raw)
	return fmt.Sprintf("certificate %q issued by %q, valid %s to %s, SHA-256 %s", c.Subject.CommonName, c.Issuer.CommonName,
		c.NotBefore.UTC().Format("2006-01-02"), c.NotAfter.UTC().Format("2006-01-02"), hex.EncodeToString(h[:]))
}

// loadOptionDevicePath reads the device path of an EFI_LOAD_OPTION
// (Boot####): Attributes (u32), FilePathListLength (u16), Description
// (UCS-2, NUL-terminated), FilePathList.
func loadOptionDevicePath(b []byte) string {
	if len(b) < 6 {
		return ""
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	i := 6
	for i+1 < len(b) && (b[i] != 0 || b[i+1] != 0) {
		i += 2
	}
	i += 2
	if i+n > len(b) || n == 0 {
		return ""
	}
	elems, err := parseDevicePath(b[i : i+n])
	if err != nil {
		return ""
	}
	return devicePathText(elems)
}

// parseDevicePath reads device path nodes up to the end node.
func parseDevicePath(b []byte) ([]tcg.EFIDevicePathElement, error) {
	var out []tcg.EFIDevicePathElement
	for len(b) >= 4 {
		typ, sub, l := b[0], b[1], int(binary.LittleEndian.Uint16(b[2:]))
		if l < 4 || l > len(b) {
			return out, fmt.Errorf("bad device path node length %d", l)
		}
		if typ == 0x7f {
			break
		}
		out = append(out, tcg.EFIDevicePathElement{Type: tcg.EFIDeviceType(typ), Subtype: sub, Data: b[4:l]})
		b = b[l:]
	}
	return out, nil
}

// devicePathText writes a device path roughly as UEFI shells do, e.g.
// PciRoot/Pci(0x1,0x1)/Sata(0)/HD(1)/\EFI\debian\shimx64.efi.
func devicePathText(elems []tcg.EFIDevicePathElement) string {
	var parts []string
	for _, e := range elems {
		d := e.Data
		switch {
		case e.Type == tcg.ACPIDevice && e.Subtype == 1:
			parts = append(parts, "PciRoot")
		case e.Type == tcg.HardwareDevice && e.Subtype == 1 && len(d) >= 2:
			parts = append(parts, fmt.Sprintf("Pci(0x%x,0x%x)", d[1], d[0]))
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x12 && len(d) >= 2:
			parts = append(parts, fmt.Sprintf("Sata(%d)", binary.LittleEndian.Uint16(d)))
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x17 && len(d) >= 4:
			parts = append(parts, fmt.Sprintf("NVMe(%d)", binary.LittleEndian.Uint32(d)))
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x02:
			parts = append(parts, "Scsi")
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x05 && len(d) >= 2:
			parts = append(parts, fmt.Sprintf("USB(%d,%d)", d[0], d[1]))
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x0b:
			parts = append(parts, "MAC")
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x0c:
			parts = append(parts, "IPv4")
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x0d:
			parts = append(parts, "IPv6")
		case e.Type == tcg.MessagingDevice && e.Subtype == 0x18:
			parts = append(parts, "Uri("+strings.TrimRight(string(d), "\x00")+")")
		case e.Type == tcg.MediaDevice && e.Subtype == 1 && len(d) >= 4:
			parts = append(parts, fmt.Sprintf("HD(%d)", binary.LittleEndian.Uint32(d)))
		case e.Type == tcg.MediaDevice && e.Subtype == 2:
			parts = append(parts, "CDROM")
		case e.Type == tcg.MediaDevice && e.Subtype == 4 && len(d)%2 == 0:
			u := make([]uint16, len(d)/2)
			for i := range u {
				u[i] = binary.LittleEndian.Uint16(d[2*i:])
			}
			parts = append(parts, strings.TrimRight(string(utf16.Decode(u)), "\x00"))
		case e.Type == tcg.MediaDevice && e.Subtype == 6 && len(d) >= 16:
			parts = append(parts, "FvFile("+guidString(d)+")")
		case e.Type == tcg.MediaDevice && e.Subtype == 7 && len(d) >= 16:
			parts = append(parts, "Fv("+guidString(d)+")")
		case e.Type == tcg.MediaDevice && e.Subtype == 8 && len(d) >= 20: // reserved u32, start u64, end u64
			parts = append(parts, fmt.Sprintf("Offset(0x%x,0x%x)", binary.LittleEndian.Uint64(d[4:]), binary.LittleEndian.Uint64(d[12:])))
		default:
			parts = append(parts, fmt.Sprintf("Node(%d,%d)", e.Type, e.Subtype))
		}
	}
	return strings.Join(parts, "/")
}

// guidString formats a little-endian EFI GUID.
func guidString(b []byte) string {
	if len(b) < 16 {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x", binary.LittleEndian.Uint32(b), binary.LittleEndian.Uint16(b[4:]),
		binary.LittleEndian.Uint16(b[6:]), b[8:10], b[10:16])
}

var gptTypes = map[string]string{
	"c12a7328-f81f-11d2-ba4b-00a0c93ec93b": "EFI System",
	"0fc63daf-8483-4772-8e79-3d69d8477de4": "Linux filesystem",
	"e6d6d379-f507-44c2-a23c-238f2a3df928": "Linux LVM",
	"0657fd6d-a4ab-43c4-84e5-0933c84b4f4f": "Linux swap",
	"4f68bce3-e8cd-4db1-96e7-fbcaf984b709": "Linux root (x86-64)",
	"ebd0a0a2-b9e5-4433-87c0-68b6b72699c7": "Microsoft basic data",
	"e3c9e316-0b5c-4db8-817d-f92df00215ae": "Microsoft reserved",
	"de94bba4-06d1-4d40-a16a-bfd50179d6ac": "Windows recovery",
	"21686148-6449-6e6f-744e-656564454649": "BIOS boot",
}

// gptDetails decodes UEFI_GPT_DATA: the partition table header (its own
// size and the size of an entry are in it), the number of partitions, the
// partitions.
func gptDetails(b []byte) []string {
	if len(b) < 92+8 || string(b[:8]) != "EFI PART" {
		return nil
	}
	hdrSize := int(binary.LittleEndian.Uint32(b[12:]))
	entrySize := int(binary.LittleEndian.Uint32(b[84:]))
	if hdrSize < 92 || hdrSize > len(b)-8 {
		hdrSize = 92
	}
	n := binary.LittleEndian.Uint64(b[hdrSize:])
	lines := []string{fmt.Sprintf("Disk %s, %d partitions", guidString(b[56:72]), n)}
	if entrySize < 128 {
		return append(lines, fmt.Sprintf("partition entry size %d is too small", entrySize))
	}
	off := hdrSize + 8
	for i := uint64(0); i < n && off+entrySize <= len(b); i++ {
		p := b[off : off+entrySize]
		off += entrySize
		typ := guidString(p[0:16])
		if name, ok := gptTypes[typ]; ok {
			typ = name
		}
		u := make([]uint16, 0, 36)
		for j := 56; j+1 < 128; j += 2 {
			c := binary.LittleEndian.Uint16(p[j:])
			if c == 0 {
				break
			}
			u = append(u, c)
		}
		lines = append(lines, fmt.Sprintf("partition %d: %q, %s, LBA %d–%d", i+1, string(utf16.Decode(u)), typ,
			binary.LittleEndian.Uint64(p[32:]), binary.LittleEndian.Uint64(p[40:])))
	}
	return lines
}

// noActionDetails: the startup locality, which the PCR 0 replay depends on.
// (go-eventlog reads the log's Spec ID event itself; it is not listed.)
func noActionDetails(b []byte) []string {
	if bytes.HasPrefix(b, []byte("StartupLocality\x00")) && len(b) >= 17 {
		return []string{fmt.Sprintf("Startup locality %d", b[16])}
	}
	return nil
}

var efiTableGUIDs = map[string]string{
	"8868e871-e4f1-11d3-bc22-0080c73c8881": "ACPI 2.0",
	"eb9d2d30-2d88-11d3-9a16-0090273fc14d": "ACPI 1.0",
	"f2fd1544-9794-4a2c-992e-e5bbcf20e394": "SMBIOS 3",
	"eb9d2d31-2d88-11d3-9a16-0090273fc14d": "SMBIOS",
}

// handoffDetails lists the configuration tables handed to the OS.
func handoffDetails(typ string, b []byte) []string {
	if typ == "EV_EFI_HANDOFF_TABLES2" {
		if len(b) < 1 || len(b) < 1+int(b[0]) {
			return nil
		}
		n := int(b[0])
		lines := []string{"Tables: " + strings.TrimRight(string(b[1:1+n]), "\x00")}
		return append(lines, handoffDetails("EV_EFI_HANDOFF_TABLES", b[1+n:])...)
	}
	if len(b) < 8 {
		return nil
	}
	n := binary.LittleEndian.Uint64(b)
	var lines []string
	for i, off := uint64(0), 8; i < n && off+24 <= len(b); i, off = i+1, off+24 {
		g := guidString(b[off : off+16])
		if name, ok := efiTableGUIDs[g]; ok {
			g = name
		}
		lines = append(lines, fmt.Sprintf("table %s at 0x%x", g, binary.LittleEndian.Uint64(b[off+16:])))
	}
	return lines
}
