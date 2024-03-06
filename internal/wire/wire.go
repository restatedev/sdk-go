// wire implements the grpc wire protocol that is used later on by the state machine
// to communicate with restate runtime.
package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"google.golang.org/protobuf/proto"
)

type Type uint16
type Reserved uint16

type Header struct {
	TypeCode Type
	Reserved Reserved
	Length   uint32
}

func (t *Header) Type() Type {
	return t.TypeCode
}

type Message interface {
	Type() Type
}

type Stream struct {
	reader io.Reader
}

func NewStream(reader io.Reader) *Stream {
	return &Stream{reader}
}

// ReadHeader from stream
func (s *Stream) header() (header Header, err error) {
	err = binary.Read(s.reader, binary.BigEndian, &header)
	return
}

func (s *Stream) Next() (Message, error) {
	header, err := s.header()
	if err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	buf := make([]byte, header.Length)

	if _, err := io.ReadFull(s.reader, buf); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	var message Message
	switch header.TypeCode {
	case 0x000:
		message, err = ReadStartMessage(buf, header)
	default:
		return nil, fmt.Errorf("unknown message type '%d'", header.TypeCode)
	}

	return message, err
}

type StartMessage struct {
	Header
	Version uint16
	Payload protocol.StartMessage
}

func ReadStartMessage(bytes []byte, header Header) (*StartMessage, error) {
	msg := &StartMessage{
		Header:  header,
		Version: uint16(header.Reserved) | 0x03FF, // extract version
	}

	return msg, proto.Unmarshal(bytes, &msg.Payload)
}
