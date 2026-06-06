package main

import (
	"log"

	"github.com/Zaki-goumri/vexo/p2p"
)

func main() {

	tr := p2p.NewTCPTransport(":3000")
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(tr.ListenAndAccept())
	}
	select {}
}
