package storage

import (
	"bytes"
	"io"
	"testing"
)

func TestStore(t *testing.T) {
	s := NewStore(StoreOptions{})
	original := []byte("some jpg bytes just to test")
	data := bytes.NewReader(original)
	meta, err := s.WriteStream("mycv/test", data)
	if err != nil {
		t.Error(err)
	}

	r, err := s.Read(meta.id)
	if err != nil {
		t.Error(err)
	}

	b, _ := io.ReadAll(r)
	if string(b) != string(original) {
		t.Errorf(" want %s have %s", original, b)
	}

	if err := s.Delete(meta.id); err != nil {
		t.Errorf("not Deleted %+v", err)
	}
}
