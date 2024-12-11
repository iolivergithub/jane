package utilities

import (
	"a10/structures"
)

func SelectEndpoint(e structures.Element, epn string) (structures.Endpoint, bool) {

	eps := e.Endpoints
	ep, ok := eps[epn]

	return ep, ok
}
