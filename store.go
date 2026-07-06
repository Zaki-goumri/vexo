package main

import (
	"io"
	"log"
	"os"

	"github.com/google/uuid"
)

type PathTransformFunc func(string) string

type StoreOptions struct {
	PathTransformFunc PathTransformFunc
}

var DefaultPathTransformFunc = func(key string) string {
	return "volume/" + key
}

type Store struct {
	StoreOpts StoreOptions
}

func NewStore(opts StoreOptions) *Store {
	return &Store{StoreOpts: opts}
}

func (s *Store) writeStream(bucket string, r io.Reader) error {
	//need to hash it or something for better search , ig i'll use b tree or something for search
	pathName := s.StoreOpts.PathTransformFunc(bucket)
	if err := os.MkdirAll(pathName, os.ModePerm); err != nil {
		return err
	}
	fileName := uuid.New().String()
	f, err := os.Create(pathName + "/" + fileName)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	log.Printf("written (%d) bytes to disk file: %s", n, fileName)

	return nil

}
