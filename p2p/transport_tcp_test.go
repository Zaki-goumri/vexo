package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTCPTransport(t *testing.T) {
	tcpOpts := TCPoptions{
		ListenAddr: ":4000",
		Handshaker: NOPHandshakeFunc,
		Decoder:    DefaultDecoder{},
	}
	tr := NewTCPTransport(tcpOpts)
	assert.Equal(t, tr.Config.ListenAddr, tcpOpts.ListenAddr)
	assert.Nil(t, tr.ListenAndAccept())
}
