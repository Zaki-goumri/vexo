package buckets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	return NewStore(meta, tmp)
}

func TestCreate(t *testing.T) {
	store := newTestStore(t)

	cfg, err := store.Create("photos")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cfg.Name != "photos" {
		t.Fatalf("got name %q, want %q", cfg.Name, "photos")
	}
	if cfg.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}

	// duplicate
	_, err = store.Create("photos")
	if !errors.Is(err, ErrBucketAlreadyExists) {
		t.Fatalf("want ErrBucketAlreadyExists, got %v", err)
	}

	// dir was created
	info, err := os.Stat(filepath.Join(store.root, "photos"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

func TestCreateInvalidName(t *testing.T) {
	store := newTestStore(t)

	tests := []string{
		"ab",       // too short
		"UPPER",    // uppercase
		"with_underscore",
		"-leading",
		"trailing-",
		"double--hyphen",
	}
	for _, name := range tests {
		_, err := store.Create(name)
		if !errors.Is(err, ErrInvalidBucketName) {
			t.Fatalf("Create(%q): want ErrInvalidBucketName, got %v", name, err)
		}
	}
}

func TestGet(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Create("photos"); err != nil {
		t.Fatal(err)
	}

	cfg, err := store.Get("photos")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cfg.Name != "photos" {
		t.Fatalf("got name %q, want %q", cfg.Name, "photos")
	}

	// missing bucket
	_, err = store.Get("ghost")
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("want ErrBucketNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, err := store.Create(name); err != nil {
			t.Fatal(err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("got %d buckets, want 3", len(list))
	}

	// bbolt returns keys sorted, so list should be alphabetical
	want := []string{"alpha", "beta", "gamma"}
	for i, cfg := range list {
		if cfg.Name != want[i] {
			t.Fatalf("list[%d]: got %q, want %q", i, cfg.Name, want[i])
		}
	}
}

func TestDeleteEmpty(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Create("photos"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("photos"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// metadata gone
	_, err := store.Get("photos")
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("want ErrBucketNotFound after delete, got %v", err)
	}

	// directory gone
	if _, err := os.Stat(filepath.Join(store.root, "photos")); !os.IsNotExist(err) {
		t.Fatalf("expected dir to be gone, got err: %v", err)
	}
}

func TestDeleteNotEmpty(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Create("photos"); err != nil {
		t.Fatal(err)
	}

	// simulate an object existing in the objects keyspace
	if err := store.meta.Put("objects", "photos/cat.jpg", []byte("{}")); err != nil {
		t.Fatal(err)
	}

	err := store.Delete("photos")
	if !errors.Is(err, ErrBucketNotEmpty) {
		t.Fatalf("want ErrBucketNotEmpty, got %v", err)
	}

	// bucket should still exist
	if _, err := store.Get("photos"); err != nil {
		t.Fatalf("bucket should still exist after rejecting delete: %v", err)
	}
}

func TestDeleteMissing(t *testing.T) {
	store := newTestStore(t)

	err := store.Delete("ghost")
	if !errors.Is(err, ErrBucketNotFound) {
		t.Fatalf("want ErrBucketNotFound, got %v", err)
	}
}