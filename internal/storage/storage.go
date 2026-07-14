package storage

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Zaki-goumri/vexo/internal/p2p"
	"github.com/google/uuid"
)

type PathTransformFunc func(string) string

type StoreOptions struct {
	PathTransformFunc PathTransformFunc
}

type MetaData struct {
	OriginalKey string
	id          string
}

type Store struct {
	volumeRoot string
	StoreOpts  StoreOptions
}

func NewStore(opts StoreOptions) *Store {
	return &Store{volumeRoot: "volume", StoreOpts: opts}
}

func NewStoreWithRoot(root string, opts StoreOptions) *Store {
	return &Store{volumeRoot: root, StoreOpts: opts}
}

func (s *Store) VolumeRoot() string {
	return s.volumeRoot
}

func (s *Store) SaveFileMetaData(meta *MetaData) error {
	return nil
}

// objectPath returns the on-disk path for a flat object id under the volume root.
func (s *Store) objectPath(id string) string {
	return filepath.Join(s.volumeRoot, id)
}

func (s *Store) WriteStream(key string, r io.Reader) (*MetaData, error) {
	if err := os.MkdirAll(s.volumeRoot, os.ModePerm); err != nil {
		return nil, err
	}
	id := uuid.New().String()
	f, err := os.Create(s.objectPath(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return nil, err
	}

	savedMeta := &MetaData{
		OriginalKey: key,
		id:          id,
	}
	if err := s.SaveFileMetaData(savedMeta); err != nil {
		return nil, err
	}
	return savedMeta, nil
}

func (s *Store) Read(id string) (io.Reader, error) {
	f, err := s.readStream(id)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, f); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *Store) Delete(id string) error {
	if err := os.Remove(s.objectPath(id)); err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	return nil
}

func (s *Store) readStream(id string) (io.ReadCloser, error) {
	return os.Open(s.objectPath(id))
}

func HandleRPC(s *Store, rpc p2p.RPC) {
	switch rpc.Command {
	case p2p.CommandStoreFile:
		_, err := s.WriteStream(rpc.Key, bytes.NewReader(rpc.Payload))
		if err != nil {
			log.Printf("store %s failed: %v", rpc.Key, err)
			return
		}
	default:
		log.Printf("unknown command %d from %s", rpc.Command, rpc.From)
	}
}
