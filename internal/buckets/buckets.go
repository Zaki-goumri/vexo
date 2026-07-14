package buckets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Zaki-goumri/vexo/internal/db"
)

var bucketNameRE = regexp.MustCompile(`^[a-z0-9-]+$`)

type BucketConfig struct {
	Name       string
	Versioning bool
	Lifecycle  []byte
	Tags       map[string]string
	CreatedAt  time.Time
	CreatedBy  string
}

type Store struct {
	meta *db.DB
	root string
}

var (
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketNotEmpty      = errors.New("bucket not empty")
	ErrInvalidBucketName   = errors.New("invalid bucket name")
)

func NewStore(meta *db.DB, root string) *Store {
	return &Store{
		meta: meta,
		root: root,
	}
}

func NewBucketConfig(name string) *BucketConfig {
	return &BucketConfig{
		Name:      name,
		CreatedAt: time.Now(),
		Tags:      map[string]string{},
	}
}

func (s *Store) Create(name string) (*BucketConfig, error) {
	if !validateName(name) {
		return nil, ErrInvalidBucketName
	}
	exists, err := s.meta.Has("buckets", name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrBucketAlreadyExists
	}
	bucket := NewBucketConfig(name)
	jsonBucket, err := json.Marshal(bucket)
	if err != nil {
		return nil, err
	}
	if err := s.meta.Put("buckets", name, jsonBucket); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(s.root, name), 0o755); err != nil {
		s.meta.Delete("buckets", name)
		return nil, fmt.Errorf("create bucket dir: %w", err)
	}
	return bucket, nil
}

func (s *Store) Get(name string) (*BucketConfig, error) {
	data, err := s.meta.Get("buckets", name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrBucketNotFound
		}
		return nil, err
	}
	var result BucketConfig
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal bucket config: %w", err)
	}
	return &result, nil
}

func (s *Store) List() ([]*BucketConfig, error) {
	var buckets []*BucketConfig
	err := s.meta.ForEach("buckets", func(k, v []byte) error {
		var cfg BucketConfig
		if err := json.Unmarshal(v, &cfg); err != nil {
			return fmt.Errorf("unmarshal bucket %q: %w", k, err)
		}
		buckets = append(buckets, &cfg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

func (s *Store) Delete(name string) error {
	if _, err := s.Get(name); err != nil {
		return err
	}
	prefix := name + "/"
	err := s.meta.ForEach("objects", func(k, v []byte) error {
		if strings.HasPrefix(string(k), prefix) {
			return ErrBucketNotEmpty
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.meta.Delete("buckets", name); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(s.root, name)); err != nil {
		return fmt.Errorf("remove bucket dir: %w", err)
	}
	return nil
}

func validateName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	if !bucketNameRE.MatchString(name) {
		return false
	}
	if !isLetterOrNumber(name[0]) || !isLetterOrNumber(name[len(name)-1]) {
		return false
	}
	if strings.Contains(name, "--") {
		return false
	}
	return true
}

func isLetterOrNumber(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9')
}