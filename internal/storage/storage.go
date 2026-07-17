package storage

import (
	"bytes"
	"compress/gzip"
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

const (
	TierHot        = "hot"
	TierInfrequent = "infrequent"
	TierCold       = "cold"
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
	meta        *db.DB
	bucketStore *buckets.Store
	volumeRoot  string
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

func (s *Store) Meta() *db.DB {
	return s.meta
}

func (s *Store) objectPath(bucket, id, tier string) string {
	switch tier {
	case TierCold:
		return filepath.Join(s.volumeRoot, bucket, ".cold", id+".gz")
	case TierInfrequent:
		return filepath.Join(s.volumeRoot, bucket, ".infrequent", id)
	default:
		return filepath.Join(s.volumeRoot, bucket, id)
	}
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
	objectPath := s.objectPath(bucket, id, TierHot)

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
		Tier:           TierHot,
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

	path := s.objectPath(bucket, meta.ID, meta.Tier)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open object file: %w", err)
	}

	var reader io.ReadCloser = f
	if meta.Tier == TierCold {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("gzip decompress: %w", err)
		}
		reader = &gzipReadCloser{gz: gz, f: f}
	}

	meta.LastAccessedAt = time.Now()
	meta.AccessCount++
	touchedJSON, err := json.Marshal(&meta)
	if err != nil {
		log.Printf("touch %s/%s: %v", bucket, key, err)
	} else if err := s.meta.Put("objects", bucket+"/"+key, touchedJSON); err != nil {
		log.Printf("touch %s/%s: %v", bucket, key, err)
	}

	return reader, &meta, nil
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

	objectPath := s.objectPath(bucket, meta.ID, meta.Tier)
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

func (s *Store) Transition(bucket, key, newTier string) (*ObjectMeta, error) {
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

	if meta.Tier == newTier {
		return &meta, nil
	}

	srcPath := s.objectPath(bucket, meta.ID, meta.Tier)
	src, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	var reader io.Reader = src
	if meta.Tier == TierCold {
		gz, err := gzip.NewReader(src)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress source: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	dstPath := s.objectPath(bucket, meta.ID, newTier)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return nil, fmt.Errorf("create tier dir: %w", err)
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("create dest: %w", err)
	}
	defer dst.Close()

	if newTier == TierCold {
		gz := gzip.NewWriter(dst)
		_, err = io.Copy(gz, reader)
		gz.Close()
	} else {
		_, err = io.Copy(dst, reader)
	}
	if err != nil {
		os.Remove(dstPath)
		return nil, fmt.Errorf("copy object: %w", err)
	}

	dst.Close()

	if err := os.Remove(srcPath); err != nil {
		log.Printf("transition: remove old file %s: %v", srcPath, err)
	}

	oldTierDir := filepath.Dir(srcPath)
	if oldTierDir != filepath.Join(s.volumeRoot, bucket) {
		entries, _ := os.ReadDir(oldTierDir)
		if len(entries) == 0 {
			os.Remove(oldTierDir)
		}
	}

	meta.Tier = newTier
	touchedJSON, err := json.Marshal(&meta)
	if err != nil {
		return nil, fmt.Errorf("marshal meta: %w", err)
	}
	if err := s.meta.Put("objects", bucket+"/"+key, touchedJSON); err != nil {
		return nil, err
	}

	return &meta, nil
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

type gzipReadCloser struct {
	gz *gzip.Reader
	f  *os.File
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gz.Read(p)
}

func (g *gzipReadCloser) Close() error {
	g.gz.Close()
	return g.f.Close()
}

