package protocols

import (
	"a10/operations"

	"a10/protocols/a10httprestv2"
	"a10/protocols/marblerun"
	"a10/protocols/netconfprotocol"
	"a10/protocols/nullprotocol"
	"a10/protocols/ratsdprotocol"
	"a10/protocols/testcontainerprotocol"
	"a10/protocols/veraisonpsaprotocol"
)

func RegisterProtocols() {
	operations.AddProtocol(a10httprestv2.Registration())
	operations.AddProtocol(nullprotocol.Registration())
	operations.AddProtocol(netconfprotocol.Registration())
	operations.AddProtocol(marblerun.Registration())
	operations.AddProtocol(testcontainerprotocol.Registration())
	operations.AddProtocol(veraisonpsaprotocol.Registration())
	operations.AddProtocol(ratsdprotocol.Registration())

}
