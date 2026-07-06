package p2p

import (
	"fmt"
	"net"
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
	OnPeer     func(Peer) error
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
	}
}

func (p *TCPPeer) Close() error {
	return p.conn.Close()
}

type TCPTransport struct {
	Config     TCPoptions
	listeners  net.Listener
	rpcChannel chan RPC
}

func NewTCPTransport(opts TCPoptions) *TCPTransport {
	return &TCPTransport{
		Config:     opts,
		rpcChannel: make(chan RPC),
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

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcChannel
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
	var err error
	defer func() {
		fmt.Printf("dropping peer connection, connection closed %s\n", err)
		conn.Close()
	}()
	peer := NewTCPPeer(conn, true)
	if err = t.Config.Handshaker(peer); err != nil {
		conn.Close()
		fmt.Printf("tcp error, connection closed %s\n", err)
		return
	}
	if t.Config.OnPeer != nil {
		if err = t.Config.OnPeer(peer); err != nil {
			return
		}
	}
	rpc := &RPC{}
	for {
		err := t.Config.Decoder.Decode(conn, rpc)
		if err != nil {
			fmt.Printf("tcp read err: %s\n", err)
			continue
		}
		rpc.From = conn.RemoteAddr()
		t.rpcChannel <- *rpc
	}
}
