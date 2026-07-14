package storage

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/p2p"
)

func newTestTransport(t *testing.T, addr string) (*p2p.TCPTransport, *Store) {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	bucketStore := buckets.NewStore(meta, tmp)
	if _, err := bucketStore.Create("my-bucket"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	s := NewStore(meta, bucketStore, tmp)
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

func waitForObjects(t *testing.T, s *Store, bucket string, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		objects, err := s.List(bucket, "")
		if err == nil && len(objects) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d objects", want)
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
		Key:     "/my-bucket/zaki/file",
		Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	waitForObjects(t, s, "my-bucket", 1, 2*time.Second)

	r, _, err := s.Get("my-bucket", "zaki/file")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("want %q, have %q", payload, b)
	}

	bucketDir := filepath.Join(s.VolumeRoot(), "my-bucket")
	entries, _ := os.ReadDir(bucketDir)
	if len(entries) != 1 {
		t.Fatalf("want 1 file on disk, got %d", len(entries))
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
			Key:     "/my-bucket/zaki/file" + string(rune('0'+i)),
			Payload: p,
		}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	waitForObjects(t, s, "my-bucket", 3, 2*time.Second)

	objects, err := s.List("my-bucket", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 3 {
		t.Fatalf("want 3 objects, got %d", len(objects))
	}

	got := map[string]bool{}
	for _, obj := range objects {
		r, _, err := s.Get("my-bucket", obj.Key)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r)
		r.Close()
		got[string(b)] = true
	}
	for _, p := range payloads {
		if !got[string(p)] {
			t.Fatalf("missing payload %q", p)
		}
	}
}