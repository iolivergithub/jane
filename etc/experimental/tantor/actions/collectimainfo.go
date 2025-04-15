package actions

import (
	"os"
)

const IMALOGLOCATION string = "/sys/kernel/security/ima/ascii_runtime_measurements"

func CollectIMALogLocation() (string, error) {
	_, err := os.Stat(IMALOGLOCATION)

	return IMALOGLOCATION, err
}
