package x3270

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"a10/configuration"
	"a10/logging"

	"github.com/racingmars/go3270"
)

func init() {
	// put the go3270 library in debug mode
	go3270.Debug = os.Stderr
}

func StartX3270(ctx context.Context) {
	port := configuration.ConfigData.X3270.Port

	fmt.Println(" -> 3270 starting")

	//start the server
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "JANE", "X3270 service listener failed to start: "+err.Error())
		fmt.Printf("X3270 service listener failed to start: %v\n", err.Error())
		return
	}

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "JANE", "X3270 tcp assertion failed: "+err.Error())
		fmt.Printf("X3270 tcp assertion failed: %v\n", err.Error())
		return
	}

	// Set a deadline for the Accept() call
	deadline := time.Now().Add(10 * time.Second)
	if err := tcpListener.SetDeadline(deadline); err != nil {
		logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "JANE", "X3270 setting deadline failed: "+err.Error())
		fmt.Printf("X3270 setting deadline failed: %v\n", err.Error())
		return
	}

	msg := fmt.Sprintf("X3270 service started on port %v", port)
	logging.MakeLogEntry("SYS", "startup", configuration.ConfigData.System.Name, "JANE", msg)

	go func() {
		<-ctx.Done()
		tcpListener.Close()
	}()

	for {
		select {
		default:
			conn, err := tcpListener.Accept()
			if err != nil {
				//fmt.Println("An error occured:", err.Error(), " Will reset timeout deadline anyway")
				deadline = time.Now().Add(10 * time.Second)
				tcpListener.SetDeadline(deadline)
			} else {
				//fmt.Println("Accepting a connection")
				go func() { handle(conn) }()
			}
		case <-ctx.Done():
			//fmt.Println("X3270 DONE SIGNAL RECEIVED")
			msg := fmt.Sprintf("X3270 graceful shutdown")
			logging.MakeLogEntry("SYS", "shutdown", configuration.ConfigData.System.Name, "X3270", msg)
			return
		}
	}
}

// handle is the handler for individual user connections.
func handle(conn net.Conn) {
	defer conn.Close()

	// Always begin new connection by negotiating the telnet options
	go3270.NegotiateTelnet(conn)

	fieldValues := make(map[string]string)

	response, err := go3270.HandleScreen(
		titlescreen,                   // the screen to display
		titlescreenrules,              // the rules to enforce
		fieldValues,                   // any field values we wish to supply
		[]go3270.AID{go3270.AIDEnter}, // the AID keys we support
		[]go3270.AID{go3270.AIDPF3},   // keys that are "exit" keys
		"errormsg",                    // the field to write error message into
		4, 20,                         // the row and column to place the cursor
		conn)
	if err != nil {
		fmt.Printf("X3270 handle screen error %v\n", err.Error())
		fmt.Println(err)
		return
	}

	fmt.Printf("Connection closed %v \n", response)
}
