package main

import (
	"log"
	"net/http"

	"github.com/Zaki-goumri/vexo/internal/api"
	"github.com/Zaki-goumri/vexo/internal/auth"
	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/console"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/iam"
	"github.com/Zaki-goumri/vexo/internal/lifecycle"
	"github.com/Zaki-goumri/vexo/internal/p2p"
	"github.com/Zaki-goumri/vexo/internal/policy"
	"github.com/Zaki-goumri/vexo/internal/scanner"
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

	policyStore := policy.NewStore(meta)
	if err := policyStore.Put("root", policy.RootPolicy); err != nil {
		log.Fatal(err)
	}

	bucketStore := buckets.NewStore(meta, "volume")
	store := storage.NewStore(meta, bucketStore, "volume")
	lcStore := lifecycle.NewStore(bucketStore)

	apiSrv := api.NewServer(bucketStore, store, "")

	authMw := &auth.Middleware{
		IAM:    iamStore,
		Policy: policyStore,
		Next:   apiSrv.Handler(),
	}

	httpSrv := &http.Server{
		Addr:    ":9090",
		Handler: authMw,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Printf("http: %v", err)
		}
	}()

	consoleSrv := console.NewServer(iamStore, policyStore, bucketStore, store, ":9091")
	go func() {
		if err := consoleSrv.ListenAndServe(); err != nil {
			log.Printf("console: %v", err)
		}
	}()

	ilScanner := scanner.New(bucketStore, store, lcStore)
	ilScanner.Start()
	defer ilScanner.Stop()

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