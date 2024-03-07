// wire implements the grpc wire protocol that is used later on by the state machine
// to communicate with restate runtime.
package wire

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/muhamadazmy/restate-sdk-go/generated/protocol"
	"google.golang.org/protobuf/proto"
)

const (
	// masks
	FlagCompleted Flag = 0x0001
	FlagAck       Flag = 0x8000

	VersionMask = 0x03FF
)
const (
	StartMessageType          Type = 0x0000
	PollInputEntryMessageType Type = 0x0400
)

type Type uint16

// Flag section of the header this can have
// a different meaning based on message type.
type Flag uint16

// this is only valid with start message
func (r Flag) version() uint16 {
	return uint16(r) & VersionMask
}

func (r Flag) Completed() bool {
	return r&FlagCompleted != 0
}

func (r Flag) Ack() bool {
	return r&FlagAck != 0
}

type Header struct {
	TypeCode Type
	Flag     Flag
	Length   uint32
}

func (t *Header) Type() Type {
	return t.TypeCode
}

type Message interface {
	Type() Type
}

type ReaderMessage struct {
	Message Message
	Err     error
}

type Reader struct {
	ch <-chan ReaderMessage
}

// Read returns next message. Easier to use when
// you need to wait on a message during a context ctx
func (r *Reader) Read(ctx context.Context) (Message, error) {
	select {
	case msg := <-r.ch:
		return msg.Message, msg.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Reader) Next() <-chan ReaderMessage {
	return r.ch
}

// Protocol implements the wire protocol to abstract receiving
// and sending messages
// Note that Protocol is not concurrent safe and it's up to the user
// to make sure it's used correctly
type Protocol struct {
	stream io.ReadWriter
}

func NewProtocol(stream io.ReadWriter) *Protocol {
	return &Protocol{stream}
}

// ReadHeader from stream
func (s *Protocol) header() (header Header, err error) {
	err = binary.Read(s.stream, binary.BigEndian, &header)
	return
}

func (s *Protocol) read() (Message, error) {
	header, err := s.header()
	if err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	buf := make([]byte, header.Length)

	if _, err := io.ReadFull(s.stream, buf); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	var message Message
	switch header.TypeCode {
	case StartMessageType:
		message, err = readStartMessage(buf, header)
	case PollInputEntryMessageType:
		message, err = readPollInputEntryMessage(buf, header)
	default:
		return nil, fmt.Errorf("unknown message type '%d'", header.TypeCode)
	}

	return message, err
}

// Reader should be called once. We could surround
// it be a sync.Once but since this is an internal
// package so we just need to make sure it's used
// correctly
func (s *Protocol) Reader(ctx context.Context) *Reader {
	ch := make(chan ReaderMessage)

	go func(ctx context.Context) {
		defer close(ch)

		for {
			msg, err := s.read()

			select {
			case ch <- ReaderMessage{Message: msg, Err: err}:
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	return &Reader{ch}
}

func (s *Protocol) Write(message proto.Message, flags ...Flag) error {
	var flag Flag
	if len(flags) > 1 {
		// code error
		panic("invalid flags, use | operator instead")
	} else if len(flags) == 1 {
		flag = flags[0]
	}

	var typ Type
	switch message.(type) {
	case *protocol.StartMessage:
		// TODO: sdk should never write this message
		typ = StartMessageType
	case *protocol.PollInputStreamEntryMessage:
		typ = PollInputEntryMessageType
	default:
		return fmt.Errorf("unknown message type")
	}

	bytes, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// sanity checks
	if len(bytes) > math.MaxUint32 {
		return fmt.Errorf("message size is too big")
	}

	header := Header{
		TypeCode: typ,
		Flag:     flag,
		Length:   uint32(len(bytes)),
	}

	if err := binary.Write(s.stream, binary.BigEndian, header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := s.stream.Write(bytes); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

type StartMessage struct {
	Header
	Version uint16
	Payload protocol.StartMessage
}

func readStartMessage(bytes []byte, header Header) (*StartMessage, error) {
	msg := &StartMessage{
		Header:  header,
		Version: header.Flag.version(),
	}

	return msg, proto.Unmarshal(bytes, &msg.Payload)
}

type PollInputEntry struct {
	Header
	Payload protocol.PollInputStreamEntryMessage
}

func readPollInputEntryMessage(bytes []byte, header Header) (*PollInputEntry, error) {
	msg := &PollInputEntry{
		Header: header,
	}

	return msg, proto.Unmarshal(bytes, &msg.Payload)
}
