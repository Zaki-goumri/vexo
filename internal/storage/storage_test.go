package storage

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	volumeDir := filepath.Join(t.TempDir(), "volume")
	s := NewStoreWithRoot(volumeDir, StoreOptions{})
	original := []byte("some jpg bytes just to test")
	data := bytes.NewReader(original)
	meta, err := s.WriteStream("mycv/test", data)
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.Read(meta.id)
	if err != nil {
		t.Fatal(err)
	}

	b, _ := io.ReadAll(r)
	if !bytes.Equal(b, original) {
		t.Errorf("want %q have %q", original, b)
	}

	if err := s.Delete(meta.id); err != nil {
		t.Errorf("not Deleted %+v", err)
	}
}