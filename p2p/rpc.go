package p2p

import "net"

type RPC struct {
	From    net.Addr
	Command Command
	Key     string
	Payload []byte
}
