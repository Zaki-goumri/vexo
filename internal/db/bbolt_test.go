package db

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db := &DB{}
	if err := db.Open(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenClose(t *testing.T) {
	db := newTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db1 := &DB{}
	if err := db1.Open(path); err != nil {
		t.Fatalf("open1: %v", err)
	}
	if err := db1.Put("buckets", "k", []byte("v")); err != nil {
		t.Fatalf("put: %v", err)
	}
	db1.Close()

	db2 := &DB{}
	if err := db2.Open(path); err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer db2.Close()

	got, err := db2.Get("buckets", "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got, []byte("v")) {
		t.Fatalf("got %q, want %q", got, "v")
	}
}

func TestPutGet(t *testing.T) {
	db := newTestDB(t)

	if err := db.Put("buckets", "photos", []byte("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := db.Get("buckets", "photos")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestGetNotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := db.Get("buckets", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetMissingBucket(t *testing.T) {
	db := newTestDB(t)

	_, err := db.Get("nonexistent", "key")
	if err == nil {
		t.Fatal("want error for missing bbolt bucket, got nil")
	}
}

func TestDelete(t *testing.T) {
	db := newTestDB(t)

	if err := db.Put("buckets", "photos", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete("buckets", "photos"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// verify gone
	_, err := db.Get("buckets", "photos")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteIdempotent(t *testing.T) {
	db := newTestDB(t)

	// delete a key that was never put — should not error
	if err := db.Delete("buckets", "ghost"); err != nil {
		t.Fatalf("delete missing key: want nil, got %v", err)
	}
}

func TestHas(t *testing.T) {
	db := newTestDB(t)

	if err := db.Put("buckets", "photos", []byte("v")); err != nil {
		t.Fatal(err)
	}

	ok, err := db.Has("buckets", "photos")
	if err != nil {
		t.Fatalf("has: %v", err)
	}
	if !ok {
		t.Fatal("want true, got false")
	}

	ok, err = db.Has("buckets", "missing")
	if err != nil {
		t.Fatalf("has missing: %v", err)
	}
	if ok {
		t.Fatal("want false, got true")
	}
}

func TestForEach(t *testing.T) {
	db := newTestDB(t)

	for _, k := range []string{"alpha", "beta", "gamma"} {
		if err := db.Put("buckets", k, []byte(k)); err != nil {
			t.Fatal(err)
		}
	}

	var keys, values []string
	err := db.ForEach("buckets", func(k, v []byte) error {
		keys = append(keys, string(k))
		values = append(values, string(v))
		return nil
	})
	if err != nil {
		t.Fatalf("foreach: %v", err)
	}

	want := []string{"alpha", "beta", "gamma"}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("keys[%d]: got %q, want %q", i, k, want[i])
		}
		if values[i] != want[i] {
			t.Fatalf("values[%d]: got %q, want %q", i, values[i], want[i])
		}
	}
}

func TestForEachEarlyStop(t *testing.T) {
	db := newTestDB(t)

	for _, k := range []string{"a", "b", "c", "d"} {
		if err := db.Put("buckets", k, []byte(k)); err != nil {
			t.Fatal(err)
		}
	}

	count := 0
	err := db.ForEach("buckets", func(k, v []byte) error {
		count++
		if count == 2 {
			return fmt.Errorf("stop")
		}
		return nil
	})
	if err == nil {
		t.Fatal("want error from early stop, got nil")
	}
	if count != 2 {
		t.Fatalf("want count 2, got %d", count)
	}
}

func TestList(t *testing.T) {
	db := newTestDB(t)

	if err := db.Put("buckets", "photos", []byte("p")); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("buckets", "videos", []byte("v")); err != nil {
		t.Fatal(err)
	}

	list, err := db.List("buckets")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d, want 2", len(list))
	}
	if !bytes.Equal(list["photos"], []byte("p")) {
		t.Fatalf("photos: got %q, want %q", list["photos"], "p")
	}
	if !bytes.Equal(list["videos"], []byte("v")) {
		t.Fatalf("videos: got %q, want %q", list["videos"], "v")
	}
}

func TestConcurrentWrites(t *testing.T) {
	db := newTestDB(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", n)
			if err := db.Put("buckets", key, []byte(key)); err != nil {
				t.Errorf("put %s: %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("k%d", i)
		got, err := db.Get("buckets", key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if !bytes.Equal(got, []byte(key)) {
			t.Fatalf("%s: got %q, want %q", key, got, key)
		}
	}
}

