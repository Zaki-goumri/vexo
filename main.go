package main

import (
	"log"

	"github.com/Zaki-goumri/vexo/internal/p2p"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func main() {
	s := storage.NewStore(storage.StoreOptions{})

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
			storage.HandleRPC(s, rpc)
		}
	}()
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}
}
