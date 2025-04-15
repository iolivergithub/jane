package actions

import (
	"os"
)

const UEFIEVENTLOGLOCATION string = "/sys/kernel/security/tpm0/binary_bios_measurements"

func CollectUEFIEventLogLocation() (string, error) {
	_, err := os.Stat(UEFIEVENTLOGLOCATION)

	return UEFIEVENTLOGLOCATION, err
}
