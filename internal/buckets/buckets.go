package buckets

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Zaki-goumri/vexo/internal/db"
)

type BucketConfig struct {
	Name        string
	Versionning bool
	LifeCycle   []byte
	Tags        map[string]string
	CreatedAt   time.Time
	CreatedBy   string
}

type Store struct {
	meta *db.DB
	root string
}

var ErrBucketNotFound = errors.New("bucket not found")
var ErrBucketAlreadyExists = errors.New("Bucket Already Exists")

func NewStore(meta *db.DB) *Store {
	return &Store{
		meta: meta,
		root: "volume",
	}
}

func (s *Store) Create(name string) (*BucketConfig, error) {
	existingBucket, err := s.Get(name)
	if exists, err := s.meta.Has("buckets", name); exists {
		return ErrBucketAlreadyExists

	}
}

func (s *Store) Get(name string) (*BucketConfig, error) {
	var result BucketConfig
	data, err := s.meta.Get("buckets", name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrBucketNotFound
		}
	}
	json.Unmarshal(data, result)
	return &result, nil
}
func (s *Store) List() ([]*BucketConfig, error)
func (s *Store) Delete(name string) error

func validateName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
}
