package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/Zaki-goumri/vexo/p2p"
)

func main() {
	s := NewStore(StoreOptions{})

	tcpOpts := p2p.TCPoptions{
		ListenAddr: ":3001",
		Handshaker: p2p.NOPHandshakeFunc,
		Decoder:    p2p.DefaultDecoder{},
		OnPeer: func(p2p.Peer) error {
			return nil
		},
	}
	tr := p2p.NewTCPTransport(tcpOpts)
	go func() {
		for rpc := range tr.Consume() {
			handleRPC(s, rpc)
		}
	}()
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}
}

func handleRPC(s *Store, rpc p2p.RPC) {
	switch rpc.Command {
	case p2p.CommandStoreFile:
		_, err := s.writeStream(rpc.Key, bytes.NewReader(rpc.Payload))
		if err != nil {
			log.Printf("store %s failed: %v", rpc.Key, err)
			return
		}
		fmt.Printf("stored key=%s from=%s (%d bytes)\n", rpc.Key, rpc.From, len(rpc.Payload))
	default:
		log.Printf("unknown command %d from %s", rpc.Command, rpc.From)
	}
}
