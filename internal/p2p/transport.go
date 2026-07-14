// Package p2p is
package p2p

// Peer is a representation of remote node
type Peer interface {
}

// Transport can be anything handles communications between the nodes of network
type Transport interface {
	ListenAndAccept() error
	Consume() <-chan RPC
}
