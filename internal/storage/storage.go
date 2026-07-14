package storage

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/p2p"
	"github.com/google/uuid"
)

var (
	ErrObjectNotFound = errors.New("object not found")
	ErrBucketNotFound = errors.New("bucket not found")
)

type ObjectMeta struct {
	ID             string    `json:"id"`
	Bucket         string    `json:"bucket"`
	Key            string    `json:"key"`
	Size           int64     `json:"size"`
	ETag           string    `json:"etag"`
	ContentType    string    `json:"contentType"`
	Tier           string    `json:"tier"`
	CreatedAt      time.Time `json:"createdAt"`
	LastAccessedAt time.Time `json:"lastAccessedAt"`
	AccessCount    int64     `json:"accessCount"`
}

type Store struct {
	meta       *db.DB
	bucketStore *buckets.Store
	volumeRoot string
}

func NewStore(meta *db.DB, bucketStore *buckets.Store, root string) *Store {
	return &Store{
		meta:        meta,
		bucketStore: bucketStore,
		volumeRoot:  root,
	}
}

func (s *Store) VolumeRoot() string {
	return s.volumeRoot
}

func (s *Store) Put(bucket, key string, r io.Reader) (*ObjectMeta, error) {
	if _, err := s.bucketStore.Get(bucket); err != nil {
		if errors.Is(err, buckets.ErrBucketNotFound) {
			return nil, ErrBucketNotFound
		}
		return nil, err
	}

	bucketDir := filepath.Join(s.volumeRoot, bucket)
	if err := os.MkdirAll(bucketDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bucket dir: %w", err)
	}

	id := uuid.New().String()
	objectPath := filepath.Join(bucketDir, id)

	f, err := os.Create(objectPath)
	if err != nil {
		return nil, fmt.Errorf("create object file: %w", err)
	}
	defer f.Close()

	hash := md5.New()
	w := io.MultiWriter(f, hash)

	n, err := io.Copy(w, r)
	if err != nil {
		os.Remove(objectPath)
		return nil, fmt.Errorf("write object: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	now := time.Now()

	meta := &ObjectMeta{
		ID:             id,
		Bucket:         bucket,
		Key:            key,
		Size:           n,
		ETag:           etag,
		Tier:           "hot",
		CreatedAt:      now,
		LastAccessedAt: now,
		AccessCount:    0,
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		os.Remove(objectPath)
		return nil, fmt.Errorf("marshal meta: %w", err)
	}

	if err := s.meta.Put("objects", bucket+"/"+key, metaJSON); err != nil {
		os.Remove(objectPath)
		return nil, err
	}

	return meta, nil
}

func (s *Store) Get(bucket, key string) (io.ReadCloser, *ObjectMeta, error) {
	metaJSON, err := s.meta.Get("objects", bucket+"/"+key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil, ErrObjectNotFound
		}
		return nil, nil, err
	}

	var meta ObjectMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return nil, nil, fmt.Errorf("unmarshal meta: %w", err)
	}

	objectPath := filepath.Join(s.volumeRoot, bucket, meta.ID)
	f, err := os.Open(objectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open object file: %w", err)
	}

	return f, &meta, nil
}

func (s *Store) Stat(bucket, key string) (*ObjectMeta, error) {
	metaJSON, err := s.meta.Get("objects", bucket+"/"+key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	var meta ObjectMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}
	return &meta, nil
}

func (s *Store) Delete(bucket, key string) error {
	metaJSON, err := s.meta.Get("objects", bucket+"/"+key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrObjectNotFound
		}
		return err
	}

	var meta ObjectMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return fmt.Errorf("unmarshal meta: %w", err)
	}

	objectPath := filepath.Join(s.volumeRoot, bucket, meta.ID)
	if err := os.Remove(objectPath); err != nil {
		return fmt.Errorf("remove object file: %w", err)
	}

	if err := s.meta.Delete("objects", bucket+"/"+key); err != nil {
		return err
	}

	return nil
}

func (s *Store) List(bucket, prefix string) ([]*ObjectMeta, error) {
	searchKey := bucket + "/"
	if prefix != "" {
		searchKey += prefix
	}

	var results []*ObjectMeta
	err := s.meta.ForEach("objects", func(k, v []byte) error {
		if !strings.HasPrefix(string(k), searchKey) {
			return nil
		}
		var meta ObjectMeta
		if err := json.Unmarshal(v, &meta); err != nil {
			return fmt.Errorf("unmarshal meta %q: %w", k, err)
		}
		results = append(results, &meta)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func splitKey(s string) (bucket, key string) {
	s = strings.TrimPrefix(s, "/")
	idx := strings.Index(s, "/")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

func HandleRPC(s *Store, rpc p2p.RPC) {
	switch rpc.Command {
	case p2p.CommandStoreFile:
		bucket, key := splitKey(rpc.Key)
		_, err := s.Put(bucket, key, bytes.NewReader(rpc.Payload))
		if err != nil {
			log.Printf("store %s/%s failed: %v", bucket, key, err)
			return
		}
	default:
		log.Printf("unknown command %d", rpc.Command)
	}
}