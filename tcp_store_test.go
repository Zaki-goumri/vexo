package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Zaki-goumri/vexo/p2p"
)

func TestTCPStoreFile(t *testing.T) {
	s := NewStore(StoreOptions{})
	tr := p2p.NewTCPTransport(p2p.TCPoptions{
		ListenAddr: ":4002",
		Handshaker: p2p.NOPHandshakeFunc,
		Decoder:    p2p.DefaultDecoder{},
	})
	if err := tr.ListenAndAccept(); err != nil {
		t.Fatal(err)
	}
	go func() {
		for rpc := range tr.Consume() {
			t.Logf("got rpc cmd=%d key=%q bytes=%d", rpc.Command, rpc.Key, len(rpc.Payload))
			handleRPC(s, rpc)
		}
	}()

	conn, err := net.Dial("tcp", "localhost:4002")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dialed %s", conn.RemoteAddr())
	defer conn.Close()

	payload := []byte("hello vexo over tcp")
	if err := p2p.EncodeMessage(conn, p2p.Message{
		Command: p2p.CommandStoreFile,
		Key:     "/cv/zaki/file",
		Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var entries []os.DirEntry
	for time.Now().Before(deadline) {
		entries, err = os.ReadDir("volume")
		if err == nil && len(entries) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 stored file, got %v (%v)", entries, err)
	}
	r, err := s.Read(entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("want %q have %q", payload, b)
	}
}
