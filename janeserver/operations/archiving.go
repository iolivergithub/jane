package operations

import (
	"a10/structures"
	"a10/utilities"
)

func createArchiveStructure(reason string) structures.Archive {
	return structures.Archive{Reason: reason, Timestamp: utilities.MakeTimestamp()}
}
