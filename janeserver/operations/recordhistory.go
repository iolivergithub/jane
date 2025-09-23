package operations

import (
	"a10/structures"
	"a10/utilities"
)

func recordInformationCreation() structures.RecordHistory {
	return structures.RecordHistory{Created: utilities.MakeTimestamp()}
}

func recordInformationUpdate(a structures.RecordHistory) structures.RecordHistory {
	a.LastUpdated = utilities.MakeTimestamp()
	return a
}

func recordInformationArchived(a structures.RecordHistory, reason string) structures.RecordHistory {
	a.Archived = utilities.MakeTimestamp()
	a.Reason = reason
	return a
}
