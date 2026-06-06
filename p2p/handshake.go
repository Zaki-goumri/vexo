package p2p

type handshakeFunc func(any) error

func NOPHandshakeFunc(any) error { return nil }
