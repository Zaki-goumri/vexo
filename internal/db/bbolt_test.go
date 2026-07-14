package db

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func TestBBolt(t *testing.T) {
	db := DB{}
	err := db.Open("")
	if err != nil {
		t.Errorf("%v error", err)
	}
	randomNumber := rand.IntN(7)
	err = db.Put(bucketNames[randomNumber], "test", []byte("test string"))
	if err != nil {
		t.Errorf("%v error", err)
	}

	result, err := db.Get(bucketNames[randomNumber], "test")
	if err != nil {
		t.Errorf("%v error", err)
	}
	fmt.Printf("result: %s\n", result)
	list, err := db.List(bucketNames[randomNumber])
	if err != nil {
		t.Errorf("%v error", err)
	}

	for key, value := range list {
		fmt.Printf("%s: %s\n", key, string(value))
	}

	has, err := db.Has(bucketNames[randomNumber], "test")
	if err != nil {
		t.Errorf("%v error", err)
	}
	fmt.Printf("%t \n", has)
	err = db.Delete(bucketNames[randomNumber], "test")
	if err != nil {
		t.Errorf("%v error", err)
	}

}
