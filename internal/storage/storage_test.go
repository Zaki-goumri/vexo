package storage

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	bucketStore := buckets.NewStore(meta, tmp)
	return NewStore(meta, bucketStore, tmp)
}

func TestPutGet(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	payload := []byte("hello vexo")
	meta, err := s.Put("photos", "cat.jpg", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if meta.Bucket != "photos" {
		t.Fatalf("bucket: got %q, want %q", meta.Bucket, "photos")
	}
	if meta.Key != "cat.jpg" {
		t.Fatalf("key: got %q, want %q", meta.Key, "cat.jpg")
	}
	if meta.Size != int64(len(payload)) {
		t.Fatalf("size: got %d, want %d", meta.Size, len(payload))
	}
	if meta.ETag == "" {
		t.Fatal("etag is empty")
	}
	if meta.Tier != "hot" {
		t.Fatalf("tier: got %q, want %q", meta.Tier, "hot")
	}

	r, gotMeta, err := s.Get("photos", "cat.jpg")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Close()

	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("want %q, have %q", payload, b)
	}
	if gotMeta.ID != meta.ID {
		t.Fatalf("id mismatch: got %q, want %q", gotMeta.ID, meta.ID)
	}
}

func TestPutMissingBucket(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Put("ghost", "file.jpg", bytes.NewReader([]byte("data")))
	if err != ErrBucketNotFound {
		t.Fatalf("want ErrBucketNotFound, got %v", err)
	}
}

func TestGetMissingObject(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	_, _, err := s.Get("photos", "missing.jpg")
	if err != ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Put("photos", "cat.jpg", bytes.NewReader([]byte("data"))); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete("photos", "cat.jpg"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, _, err := s.Get("photos", "cat.jpg")
	if err != ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound after delete, got %v", err)
	}
}

func TestDeleteMissing(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	err := s.Delete("photos", "ghost.jpg")
	if err != ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}
}

func TestListWithPrefix(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"cat.jpg", "dog.jpg", "bird.png"} {
		if _, err := s.Put("photos", key, bytes.NewReader([]byte(key))); err != nil {
			t.Fatal(err)
		}
	}

	all, err := s.List("photos", "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 objects, got %d", len(all))
	}

	jpegs, err := s.List("photos", "")
	if err != nil {
		t.Fatalf("list jpegs: %v", err)
	}
	gotJpg := 0
	for _, m := range jpegs {
		if m.Key != "bird.png" {
			gotJpg++
		}
	}
	if gotJpg != 2 {
		t.Fatalf("want 2 jpg objects, got %d", gotJpg)
	}
}