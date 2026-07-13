package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/Zaki-goumri/vexo/p2p"
)

const usage = `vexo client
  put <addr> <file> <key>   upload file to server at addr (e.g. localhost:3001)`

func main() {
	if len(os.Args) < 5 || os.Args[1] != "put" {
		fmt.Println(usage)
		os.Exit(1)
	}
	addr, file, key := os.Args[2], os.Args[3], os.Args[4]

	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close()

	if err := p2p.EncodeMessage(conn, p2p.Message{
		Command: p2p.CommandStoreFile,
		Key:     key,
		Payload: data,
	}); err != nil {
		log.Fatalf("send: %v", err)
	}
	fmt.Printf("uploaded %s -> %s  key=%s (%d bytes)\n", file, addr, key, len(data))
}
