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
	// The go3270 debug output shows every datastream, including the data on
	// screen, so it is only switched on when asked for.
	if os.Getenv("JANE_X3270_DEBUG") != "" {
		go3270.Debug = os.Stderr
	}
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

// handle runs one user's 3270 session: telnet negotiation, then the screens,
// starting from the primary option menu.
func handle(conn net.Conn) {
	defer conn.Close()
	serve(conn, dbStore{})
}

// serve negotiates tn3270 on conn and runs the screens against s.
func serve(conn net.Conn, s store) {
	dev, err := go3270.NegotiateTelnet(conn)
	if err != nil {
		fmt.Printf("X3270 telnet negotiation failed from %v: %v\n", conn.RemoteAddr(), err)
		return
	}

	if err := go3270.RunTransactions(conn, dev, menuTx(s), nil); err != nil {
		fmt.Printf("X3270 session from %v ended: %v\n", conn.RemoteAddr(), err)
	}
}
