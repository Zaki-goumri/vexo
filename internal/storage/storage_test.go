package storage

import (
	"bytes"
	"io"
	"os"
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

func TestAccessTracking(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	meta, err := s.Put("photos", "track.jpg", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}
	if meta.AccessCount != 0 {
		t.Fatalf("initial accessCount: want 0, got %d", meta.AccessCount)
	}

	r1, m1, err := s.Get("photos", "track.jpg")
	if err != nil {
		t.Fatal(err)
	}
	r1.Close()
	if m1.AccessCount != 1 {
		t.Fatalf("after 1st get: want accessCount 1, got %d", m1.AccessCount)
	}
	if !m1.LastAccessedAt.After(meta.CreatedAt) && !m1.LastAccessedAt.Equal(meta.LastAccessedAt) {
		t.Fatal("lastAccessedAt should be >= createdAt on 1st get")
	}

	r2, m2, err := s.Get("photos", "track.jpg")
	if err != nil {
		t.Fatal(err)
	}
	r2.Close()
	if m2.AccessCount != 2 {
		t.Fatalf("after 2nd get: want accessCount 2, got %d", m2.AccessCount)
	}
	if !m2.LastAccessedAt.After(m1.LastAccessedAt) {
		t.Fatal("lastAccessedAt should increase on 2nd get")
	}

	stat, err := s.Stat("photos", "track.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if stat.AccessCount != 2 {
		t.Fatalf("stat should see accessCount 2, got %d", stat.AccessCount)
	}
}

func TestTransitionHotToCold(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	payload := bytes.Repeat([]byte("compressible data "), 100)
	meta, err := s.Put("photos", "cat.jpg", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	hotPath := filepath.Join(s.VolumeRoot(), "photos", meta.ID)
	if _, err := os.Stat(hotPath); err != nil {
		t.Fatalf("hot file should exist: %v", err)
	}

	updated, err := s.Transition("photos", "cat.jpg", TierCold)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if updated.Tier != TierCold {
		t.Fatalf("tier: got %q, want %q", updated.Tier, TierCold)
	}

	if _, err := os.Stat(hotPath); !os.IsNotExist(err) {
		t.Fatal("hot file should be gone after transition")
	}

	coldPath := filepath.Join(s.VolumeRoot(), "photos", ".cold", meta.ID+".gz")
	if _, err := os.Stat(coldPath); err != nil {
		t.Fatalf("cold file should exist: %v", err)
	}

	info, _ := os.Stat(coldPath)
	if info.Size() >= int64(len(payload)) {
		t.Fatalf("gzip should be smaller: cold=%d, raw=%d", info.Size(), len(payload))
	}
}

func TestGetAfterColdTransition(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	payload := []byte("hello vexo, this is compressible text data for testing")
	_, err := s.Put("photos", "cat.jpg", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Transition("photos", "cat.jpg", TierCold); err != nil {
		t.Fatal(err)
	}

	r, _, err := s.Get("photos", "cat.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("decompressed data mismatch: want %q, got %q", payload, b)
	}
}

func TestTransitionColdToHot(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	payload := []byte("some data to compress then decompress")
	meta, err := s.Put("photos", "doc.txt", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Transition("photos", "doc.txt", TierCold); err != nil {
		t.Fatal(err)
	}

	updated, err := s.Transition("photos", "doc.txt", TierHot)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tier != TierHot {
		t.Fatalf("tier: got %q, want %q", updated.Tier, TierHot)
	}

	hotPath := filepath.Join(s.VolumeRoot(), "photos", meta.ID)
	if _, err := os.Stat(hotPath); err != nil {
		t.Fatalf("hot file should exist after promote: %v", err)
	}

	coldPath := filepath.Join(s.VolumeRoot(), "photos", ".cold", meta.ID+".gz")
	if _, err := os.Stat(coldPath); !os.IsNotExist(err) {
		t.Fatal("cold .gz should be gone after promote")
	}

	r, _, err := s.Get("photos", "doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("data mismatch after promote: want %q, got %q", payload, b)
	}
}

func TestTransitionHotToInfrequent(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	payload := []byte("some infrequent-tier data")
	meta, err := s.Put("photos", "cat.jpg", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.Transition("photos", "cat.jpg", TierInfrequent)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tier != TierInfrequent {
		t.Fatalf("tier: got %q, want %q", updated.Tier, TierInfrequent)
	}

	ifPath := filepath.Join(s.VolumeRoot(), "photos", ".infrequent", meta.ID)
	if _, err := os.Stat(ifPath); err != nil {
		t.Fatalf("infrequent file should exist: %v", err)
	}

	hotPath := filepath.Join(s.VolumeRoot(), "photos", meta.ID)
	if _, err := os.Stat(hotPath); !os.IsNotExist(err) {
		t.Fatal("hot file should be gone")
	}

	r, _, err := s.Get("photos", "cat.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, payload) {
		t.Fatalf("data mismatch: want %q, got %q", payload, b)
	}
}

func TestTransitionSameTierNoop(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	_, err := s.Put("photos", "cat.jpg", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.Transition("photos", "cat.jpg", TierHot)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Tier != TierHot {
		t.Fatalf("tier should still be hot: %q", updated.Tier)
	}
}

func TestDeleteColdObject(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	meta, err := s.Put("photos", "cat.jpg", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Transition("photos", "cat.jpg", TierCold); err != nil {
		t.Fatal(err)
	}

	coldPath := filepath.Join(s.VolumeRoot(), "photos", ".cold", meta.ID+".gz")
	if _, err := os.Stat(coldPath); err != nil {
		t.Fatal("cold file should exist before delete")
	}

	if err := s.Delete("photos", "cat.jpg"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(coldPath); !os.IsNotExist(err) {
		t.Fatal("cold .gz should be gone after delete")
	}

	_, err = s.Stat("photos", "cat.jpg")
	if err != ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}
}

func TestStatShowsUpdatedTier(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.bucketStore.Create("photos"); err != nil {
		t.Fatal(err)
	}

	_, err := s.Put("photos", "cat.jpg", bytes.NewReader([]byte("data")))
	if err != nil {
		t.Fatal(err)
	}

	s.Transition("photos", "cat.jpg", TierCold)

	stat, err := s.Stat("photos", "cat.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if stat.Tier != TierCold {
		t.Fatalf("stat tier: got %q, want %q", stat.Tier, TierCold)
	}

	objects, err := s.List("photos", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || objects[0].Tier != TierCold {
		t.Fatalf("list tier: got %+v", objects)
	}
}