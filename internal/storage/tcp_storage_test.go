package storage

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Zaki-goumri/vexo/internal/p2p"
)

func newTestTransport(t *testing.T, addr string) (*p2p.TCPTransport, *Store) {
	t.Helper()
	volumeDir := filepath.Join(t.TempDir(), "volume")
	s := NewStoreWithRoot(volumeDir, StoreOptions{})
	tr := p2p.NewTCPTransport(p2p.TCPoptions{
		ListenAddr: addr,
		Handshaker: p2p.NOPHandshakeFunc,
		Decoder:    p2p.DefaultDecoder{},
	})
	if err := tr.ListenAndAccept(); err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for rpc := range tr.Consume() {
			HandleRPC(s, rpc)
		}
	}()
	return tr, s
}

func TestTCPStoreFile(t *testing.T) {
	_, s := newTestTransport(t, ":4002")

	conn, err := net.Dial("tcp", "localhost:4002")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte("hello vexo over tcp")
	if err := p2p.EncodeMessage(conn, p2p.Message{
		Command: p2p.CommandStoreFile,
		Key:     "/cv/zaki/file",
		Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	name := waitForFile(t, s.VolumeRoot(), 2*time.Second)
	if name == "" {
		t.Fatal("no file written before timeout")
	}

	r, err := s.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("want %q have %q", payload, b)
	}
}

func TestTCPStoreMultipleFiles(t *testing.T) {
	_, s := newTestTransport(t, ":4003")

	conn, err := net.Dial("tcp", "localhost:4003")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payloads := [][]byte{
		[]byte("first file"),
		[]byte("second file"),
		[]byte("third file"),
	}
	for i, p := range payloads {
		if err := p2p.EncodeMessage(conn, p2p.Message{
			Command: p2p.CommandStoreFile,
			Key:     "/cv/zaki/file",
			Payload: p,
		}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	files := waitForFiles(t, s.VolumeRoot(), 3, 2*time.Second)
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %d", len(files))
	}

	got := map[string]bool{}
	for _, name := range files {
		r, err := s.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r)
		got[string(b)] = true
	}
	for _, p := range payloads {
		if !got[string(p)] {
			t.Fatalf("missing payload %q", p)
		}
	}
}

func waitForFile(t *testing.T, dir string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) >= 1 {
			return entries[0].Name()
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}

func waitForFiles(t *testing.T, dir string, want int, timeout time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) >= want {
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			return names
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}