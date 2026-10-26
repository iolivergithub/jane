//go:build windows

package uefi

import (
	"fmt"
	"os"
	"unsafe"

	utilities "ta10/common"

	"golang.org/x/sys/windows"
)

// On Windows there is no securityfs: the TCG event log of the current boot
// comes from TPM Base Services, Tbsi_Get_TCG_Log_Ex (Windows 10 1803 and
// later) (#122). It is the same crypto-agile TCG log as on Linux, with
// Windows' own events in PCRs 11-14. Best effort: if TBS refuses, the error
// is returned.
var (
	tbsDLL          = windows.NewLazySystemDLL("tbs.dll")
	procGetTCGLogEx = tbsDLL.NewProc("Tbsi_Get_TCG_Log_Ex")
)

// TBS_TCGLOG_SRTM_CURRENT: PCRs 0-15, current boot or resume
const tbsTCGLogSRTMCurrent = 0

func readEventLog(requested string) ([]byte, error) {
	if utilities.IsUnsafe() && requested != "" && requested != "<nil>" {
		return os.ReadFile(GetEventLogLocation(requested))
	}
	fmt.Println("UEFI Log requested on Windows: reading it from TPM Base Services")
	if err := procGetTCGLogEx.Find(); err != nil {
		return nil, fmt.Errorf("Tbsi_Get_TCG_Log_Ex is not available (needs Windows 10 1803 or later): %v", err)
	}

	// first call: ask for the size
	var size uint32
	procGetTCGLogEx.Call(uintptr(tbsTCGLogSRTMCurrent), 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return nil, fmt.Errorf("Tbsi_Get_TCG_Log_Ex returned no event log")
	}

	buf := make([]byte, size)
	r, _, _ := procGetTCGLogEx.Call(uintptr(tbsTCGLogSRTMCurrent), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return nil, fmt.Errorf("Tbsi_Get_TCG_Log_Ex failed: TBS_RESULT 0x%08x", uint32(r))
	}
	return buf[:size], nil
}
