package p2p

import "io"

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type DefaultDecoder struct{}

func (DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	m, err := DecodeMessage(r)
	if err != nil {
		return err
	}
	msg.Command = m.Command
	msg.Key = m.Key
	msg.Payload = m.Payload
	return nil
}
