package webui

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/google/go-eventlog/tcg"
)

// The deeper decoding of the test log (#135): Secure Boot keys, the
// authority, image device paths, the Spec ID event, the firmware version.
func TestEventLogDeeperDecoding(t *testing.T) {
	v := EventLogBody(eventLogClaimBody(t))
	if v.Error != "" {
		t.Fatal(v.Error)
	}
	var all []string
	for _, b := range v.Banks {
		if b.Name != v.DefaultBank {
			continue
		}
		for _, e := range b.Events {
			all = append(all, e.Description)
			all = append(all, e.Details...)
		}
	}
	text := strings.Join(all, "\n")
	for _, want := range []string{
		"SecureBoot: Enabled (1)",
		"PK: 1 certificates",
		"certificate ",
		"Verified by certificate",
		"Device path: ",
		"Firmware version: GCE Virtual Firmware v1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("decoded log lacks %q", want)
		}
	}
}

func TestDevicePathText(t *testing.T) {
	file := utf16.Encode([]rune(`\EFI\debian\shimx64.efi` + "\x00"))
	fb := make([]byte, 2*len(file))
	for i, c := range file {
		binary.LittleEndian.PutUint16(fb[2*i:], c)
	}
	elems := []tcg.EFIDevicePathElement{
		{Type: tcg.ACPIDevice, Subtype: 1, Data: make([]byte, 8)},
		{Type: tcg.HardwareDevice, Subtype: 1, Data: []byte{1, 2}},
		{Type: tcg.MediaDevice, Subtype: 1, Data: []byte{3, 0, 0, 0}},
		{Type: tcg.MediaDevice, Subtype: 4, Data: fb},
	}
	if got, want := devicePathText(elems), `PciRoot/Pci(0x2,0x1)/HD(3)/\EFI\debian\shimx64.efi`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestGPTDetails(t *testing.T) {
	b := make([]byte, 92+8+128)
	copy(b, "EFI PART")
	binary.LittleEndian.PutUint32(b[12:], 92)
	binary.LittleEndian.PutUint32(b[84:], 128)
	binary.LittleEndian.PutUint64(b[92:], 1)
	p := b[100:]
	copy(p, []byte{0x28, 0x73, 0x2a, 0xc1, 0x1f, 0xf8, 0xd2, 0x11, 0xba, 0x4b, 0x00, 0xa0, 0xc9, 0x3e, 0xc9, 0x3b}) // EFI System
	binary.LittleEndian.PutUint64(p[32:], 2048)
	binary.LittleEndian.PutUint64(p[40:], 1050623)
	copy(p[56:], []byte{'E', 0, 'S', 0, 'P', 0})
	lines := gptDetails(b)
	if len(lines) != 2 || !strings.Contains(lines[1], `"ESP", EFI System, LBA 2048–1050623`) {
		t.Errorf("got %q", lines)
	}
	if gptDetails([]byte("short")) != nil {
		t.Error("garbage decoded")
	}
	binary.LittleEndian.PutUint32(b[84:], 0) // entry size 0: must not panic
	if lines := gptDetails(b); len(lines) != 2 || !strings.Contains(lines[1], "too small") {
		t.Errorf("entry size 0: got %q", lines)
	}
}

func TestStartupLocality(t *testing.T) {
	if got := noActionDetails(append([]byte("StartupLocality\x00"), 3)); len(got) != 1 || got[0] != "Startup locality 3" {
		t.Errorf("got %q", got)
	}
}

func TestEventDetailsBounded(t *testing.T) {
	// malformed data never panics and gives nothing
	for _, typ := range []string{"EV_EFI_VARIABLE_DRIVER_CONFIG", "EV_EFI_VARIABLE_AUTHORITY", "EV_EFI_PLATFORM_FIRMWARE_BLOB2",
		"EV_EFI_BOOT_SERVICES_APPLICATION", "EV_EFI_GPT_EVENT", "EV_NO_ACTION", "EV_EFI_HANDOFF_TABLES", "EV_EFI_HANDOFF_TABLES2"} {
		for _, d := range [][]byte{nil, {1}, {255, 255, 255, 255, 255, 255, 255, 255, 255}} {
			_ = eventDetails(typ, 7, d)
		}
	}
}
