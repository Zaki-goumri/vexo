package db

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

var ErrNotFound = errors.New("db: key not found")

var bucketNames = []string{
	"buckets",
	"objects",
	"users",
	"groups",
	"policies",
	"tiers",
}

func BucketNamesBytes() [][]byte {
	names := make([][]byte, len(bucketNames))
	for i, name := range bucketNames {
		names[i] = []byte(name)
	}
	return names
}

type DB struct {
	bolt *bbolt.DB
}

const defaultDBPath = "../../volume/.volume.meta.db"

func (d *DB) Open(path string) error {
	if path == "" {
		path = defaultDBPath
	}
	boltDB, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 60 * time.Second})
	if err != nil {
		return fmt.Errorf("error in db %s:%w", path, err)
	}
	ByteNames := BucketNamesBytes()

	err = boltDB.Update(func(tx *bbolt.Tx) error {
		for _, name := range ByteNames {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		boltDB.Close()
		return err
	}
	d.bolt = boltDB
	return nil
}

func (d *DB) Close() error {
	if d.bolt == nil {
		return nil
	}
	return d.bolt.Close()
}

func (d *DB) Put(bucket, key string, value []byte) error {
	return d.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		return b.Put([]byte(key), value)
	})
}

func (d *DB) Get(bucket, key string) ([]byte, error) {
	var result []byte
	err := d.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		v := b.Get([]byte(key))
		if v == nil {
			return ErrNotFound
		}
		result = bytes.Clone(v)
		return nil
	})
	return result, err
}

func (d *DB) Delete(bucket, key string) error {
	return d.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		return b.Delete([]byte(key))
	})
}

func (d *DB) List(bucket string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	if err := d.ForEach(bucket, func(k, v []byte) error {
		out[string(k)] = v
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) Has(bucket, key string) (bool, error) {
	var found bool
	err := d.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		found = b.Get([]byte(key)) != nil
		return nil
	})
	return found, err
}

func (d *DB) ForEach(bucket string, fn func(k, v []byte) error) error {
	return d.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if err := fn(bytes.Clone(k), bytes.Clone(v)); err != nil {
				return err
			}
		}
		return nil
	})
}
