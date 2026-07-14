package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Command byte

const (
	CommandStoreFile Command = 1
	CommandGetFile   Command = 2
	CommandDelFile   Command = 3
)

type Message struct {
	Command Command
	Key     string
	Payload []byte
}

func (m Message) Bytes() ([]byte, error) {
	if len(m.Key) > 0xFFFFFFFF {
		return nil, fmt.Errorf("key too long")
	}
	if len(m.Payload) > 0xFFFFFFFF {
		return nil, fmt.Errorf("payload too large")
	}
	buf := make([]byte, 1+4+len(m.Key)+4+len(m.Payload))
	buf[0] = byte(m.Command)
	binary.LittleEndian.PutUint32(buf[1:5], uint32(len(m.Key)))
	copy(buf[5:5+len(m.Key)], m.Key)
	off := 5 + len(m.Key)
	binary.LittleEndian.PutUint32(buf[off:off+4], uint32(len(m.Payload)))
	copy(buf[off+4:], m.Payload)
	return buf, nil
}

func EncodeMessage(w io.Writer, m Message) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func DecodeMessage(r io.Reader) (Message, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Message{}, err
	}
	keyLen := binary.LittleEndian.Uint32(hdr[1:5])

	key := make([]byte, keyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return Message{}, err
	}

	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return Message{}, err
	}
	payloadLen := binary.LittleEndian.Uint32(lenBuf[:])

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Message{}, err
	}
	return Message{Command: Command(hdr[0]), Key: string(key), Payload: payload}, nil
}
