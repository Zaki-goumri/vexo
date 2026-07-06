package main

import (
	"fmt"
	"log"

	"github.com/Zaki-goumri/vexo/p2p"
)

func main() {
	tcpOpts := p2p.TCPoptions{
		ListenAddr: ":3001",
		Handshaker: p2p.NOPHandshakeFunc,
		Decoder:    p2p.DefaultDecoder{},
		OnPeer: func(p2p.Peer) error {
			fmt.Errorf("failed the oppent func")
			return nil
		},
	}
	tr := p2p.NewTCPTransport(tcpOpts)
	go func() {
		for {
			msg := <-tr.Consume()
			fmt.Printf("%+v\n", msg)
		}
	}()
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(tr.ListenAndAccept())
	}
	select {}
}
