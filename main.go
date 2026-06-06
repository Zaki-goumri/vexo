package main

import (
	"log"

	"github.com/Zaki-goumri/vexo/p2p"
)

func main() {
	tcpOpts := p2p.TCPoptions{
		ListenAddr: ":3000",
		Handshaker: p2p.NOPHandshakeFunc,
		Decoder:    p2p.DefaultDecoder{},
	}
	tr := p2p.NewTCPTransport(tcpOpts)
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(tr.ListenAndAccept())
	}
	select {}
}
