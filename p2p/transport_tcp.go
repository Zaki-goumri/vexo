package p2p

import (
	"fmt"
	"net"
	"sync"
)

type TCPPeer struct {
	conn net.Conn
	//if we dial a connection and retreive a conn -> outbound == true
	//if we accept  and retreive a conn -> outbound == false
	outbound bool
}

type TCPoptions struct {
	ShakeHands handshakeFunc
	Decoder    Decoder
	Handshaker handshakeFunc
	ListenAddr string
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
	}
}

type TCPTransport struct {
	Config    TCPoptions
	listeners net.Listener
	mu        sync.RWMutex
	peers     map[net.Addr]Peer
}

func NewTCPTransport(opts TCPoptions) *TCPTransport {
	return &TCPTransport{
		Config: opts,
	}

}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listeners, err = net.Listen("tcp", t.Config.ListenAddr)
	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listeners.Accept()
		if err != nil {
			fmt.Printf("tcp transport couldnt accept :%s\n ", err)
		}
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	peer := NewTCPPeer(conn, true)
	if err := t.Config.Handshaker(peer); err != nil {
		conn.Close()
		fmt.Printf("tcp error, connection closed %s\n", err)
		return
	}
	msg := &Message{}
	lenDecodeError := 0
	for {
		if err := t.Config.Decoder.Decode(conn, msg); err != nil {
			lenDecodeError++
			if lenDecodeError == 5 {
				fmt.Printf("tcp err: %s\n", err)
				continue
			}
		}
		msg.From = conn.RemoteAddr()
		fmt.Printf("message %+v\n", msg)
	}
}
