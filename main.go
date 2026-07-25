package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Zaki-goumri/vexo/internal/api"
	"github.com/Zaki-goumri/vexo/internal/auth"
	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/config"
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
	cfg := config.Load()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	meta := &db.DB{}
	if err := meta.Open(filepath.Join(cfg.DataDir, ".vexo.meta.db")); err != nil {
		log.Fatal(err)
	}
	defer meta.Close()

	iamStore := iam.NewStore(meta)
	if err := iamStore.BootstrapRoot(cfg.DataDir); err != nil {
		log.Fatal(err)
	}

	policyStore := policy.NewStore(meta)
	if err := policyStore.Put("root", policy.RootPolicy); err != nil {
		log.Fatal(err)
	}

	bucketStore := buckets.NewStore(meta, cfg.DataDir)
	store := storage.NewStore(meta, bucketStore, cfg.DataDir)
	lcStore := lifecycle.NewStore(bucketStore)

	shareSecret, err := console.LoadOrCreateShareSecret(cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	shareSigner := console.NewShareSigner(shareSecret)

	apiSrv := api.NewServer(bucketStore, store, "")

	authMw := &auth.Middleware{
		IAM:    iamStore,
		Policy: policyStore,
		Next:   apiSrv.Handler(),
	}

	httpSrv := &http.Server{
		Addr:    cfg.APIAddr,
		Handler: authMw,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Printf("http: %v", err)
		}
	}()

	consoleSrv := console.NewServer(iamStore, policyStore, bucketStore, store, shareSigner, cfg.ConsoleAddr)
	go func() {
		if err := consoleSrv.ListenAndServe(); err != nil {
			log.Printf("console: %v", err)
		}
	}()

	ilScanner := scanner.New(bucketStore, store, lcStore)
	ilScanner.Start()
	defer ilScanner.Stop()

	tcpOpts := p2p.TCPoptions{
		ListenAddr: cfg.P2PAddr,
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