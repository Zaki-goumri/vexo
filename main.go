package main

import (
	"log"

	"github.com/Zaki-goumri/vexo/internal/api"
	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/iam"
	"github.com/Zaki-goumri/vexo/internal/p2p"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func main() {
	meta := &db.DB{}
	if err := meta.Open("volume/.vexo.meta.db"); err != nil {
		log.Fatal(err)
	}
	defer meta.Close()

	iamStore := iam.NewStore(meta)
	if err := iamStore.BootstrapRoot("volume"); err != nil {
		log.Fatal(err)
	}

	bucketStore := buckets.NewStore(meta, "volume")
	store := storage.NewStore(meta, bucketStore, "volume")

	httpSrv := api.NewServer(bucketStore, store, ":9000")
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Printf("http: %v", err)
		}
	}()

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
			storage.HandleRPC(store, rpc)
		}
	}()
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}
}