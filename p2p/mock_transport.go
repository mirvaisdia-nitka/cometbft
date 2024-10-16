package p2p

import (
	"net"
	"time"

	na "github.com/cometbft/cometbft/p2p/netaddress"
)

type mockTransport struct {
	ln   net.Listener
	addr na.NetAddress
}

func (t *mockTransport) Listen(addr na.NetAddress) error {
	ln, err := net.Listen("tcp", addr.DialString())
	if err != nil {
		return err
	}
	t.addr = addr
	t.ln = ln
	return nil
}

// NetAddress returns the NetAddress of the local node.
func (t *mockTransport) NetAddress() na.NetAddress {
	return t.addr
}

// Accept waits for and returns the next connection to the local node.
func (t *mockTransport) Accept() (net.Conn, *na.NetAddress, error) {
	c, err := t.ln.Accept()
	return c, nil, err
}

// Dial dials the given address and returns a connection.
func (t *mockTransport) Dial(addr na.NetAddress) (net.Conn, error) {
	return addr.DialTimeout(time.Second)
}

// Cleanup any resources associated with the given connection.
//
// Must be run when the peer is dropped for any reason.
func (t *mockTransport) Cleanup(conn net.Conn) error {
	return nil
}
