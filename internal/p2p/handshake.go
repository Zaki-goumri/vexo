package p2p

type handshakeFunc func(Peer) error

func NOPHandshakeFunc(Peer) error { return nil }
