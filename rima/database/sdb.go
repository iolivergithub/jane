package database

import (
	"encoding/csv"
	"fmt"
	"os"

	"rima/configuration"
)

type sdbKey struct {
	Eid string
	Epn string
	Pol string
}

var SDB map[sdbKey]string

func SetupSDB() {
	SDB = make(map[sdbKey]string)

	// Load in DB
	fmt.Printf("loading %v\n", configuration.ConfigData.DBFile)

	f, err := os.Open(configuration.ConfigData.DBFile)
	if err != nil {
		panic(fmt.Sprintf("DB file %v does not exist.\n", configuration.ConfigData.DBFile))
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	dbdata, err := csvReader.ReadAll()
	if err != nil {
		panic(fmt.Sprintf("DB file is corrupt. Error is %v\n", err.Error()))
	}

	populateSDB(dbdata)

	fmt.Printf("SBD has %v entries\n", DBSize())
}

// Format of DB is   ElementItemID, Policy identifier, script name
func populateSDB(data [][]string) {
	for j, line := range data {
		fmt.Printf(" Entry #%v is %v %v %v\n", j, line[0], line[1], line[2], line[3])
		SDB[sdbKey{line[0], line[1], line[2]}] = line[3]
	}
}

func GetEntry(eid string, epn string, pol string) (string, bool) {
	val, ok := SDB[sdbKey{eid, epn, pol}]
	return val, ok
}

func DBSize() int {
	return len(SDB)
}
