// wire implements the grpc wire protocol that is used later on by the state machine
// to communicate with restate runtime.
package wire

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/muhamadazmy/restate-sdk-go/generated/proto/protocol"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

const (
	// masks
	FlagCompleted Flag = 0x0001
	FlagAck       Flag = 0x8000

	VersionMask = 0x03FF
)
const (
	// control
	StartMessageType      Type = 0x0000
	CompletionMessageType Type = 0x0000 + 1
	ErrorMessageType      Type = 0x0000 + 3
	EndMessageType        Type = 0x0000 + 5

	// Input/Output
	PollInputEntryMessageType    Type = 0x0400
	OutputStreamEntryMessageType Type = 0x0400 + 1

	// State
	GetStateEntryMessageType      Type = 0x0800
	SetStateEntryMessageType      Type = 0x0800 + 1
	ClearStateEntryMessageType    Type = 0x0800 + 2
	ClearAllStateEntryMessageType Type = 0x0800 + 3
	GetStateKeysEntryMessageType  Type = 0x0800 + 4

	//SysCalls
	SleepEntryMessageType            Type = 0x0C00
	InvokeEntryMessageType           Type = 0x0C00 + 1
	BackgroundInvokeEntryMessageType Type = 0x0C00 + 2
)

type Type uint16

func (t Type) String() string {
	return fmt.Sprintf("0x%04X", uint16(t))
}

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

func (t *Header) Flags() Flag {
	return t.Flag
}

type Message interface {
	Type() Type
	Flags() Flag
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

func (s *Protocol) Read() (Message, error) {
	header, err := s.header()
	if err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	buf := make([]byte, header.Length)

	if _, err := io.ReadFull(s.stream, buf); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	builder, ok := builders[header.TypeCode]
	if !ok {
		return nil, fmt.Errorf("unknown message type '%d'", header.TypeCode)
	}

	log.Trace().Stringer("type", header.TypeCode).Msg("received message")
	return builder(header, buf)

}

func (s *Protocol) Write(message proto.Message, flags ...Flag) error {
	var flag Flag
	if len(flags) > 1 {
		// code error
		panic("invalid flags, use | operator instead")
	} else if len(flags) == 1 {
		flag = flags[0]
	}

	// all possible types sent by the sdk
	var typ Type
	switch message.(type) {
	case *protocol.StartMessage:
		// TODO: sdk should never write this message
		typ = StartMessageType
	case *protocol.PollInputStreamEntryMessage:
		typ = PollInputEntryMessageType
	case *protocol.OutputStreamEntryMessage:
		typ = OutputStreamEntryMessageType
	case *protocol.ErrorMessage:
		typ = ErrorMessageType
	case *protocol.EndMessage:
		typ = EndMessageType
	case *protocol.GetStateEntryMessage:
		typ = GetStateEntryMessageType
	case *protocol.SetStateEntryMessage:
		typ = SetStateEntryMessageType
	case *protocol.ClearStateEntryMessage:
		typ = ClearStateEntryMessageType
	case *protocol.ClearAllStateEntryMessage:
		typ = ClearAllStateEntryMessageType
	case *protocol.SleepEntryMessage:
		typ = SleepEntryMessageType
	case *protocol.InvokeEntryMessage:
		typ = InvokeEntryMessageType
	case *protocol.BackgroundInvokeEntryMessage:
		typ = BackgroundInvokeEntryMessageType
	case *protocol.GetStateKeysEntryMessage:
		typ = GetStateKeysEntryMessageType
	default:
		return fmt.Errorf("can not send message of unknown message type")
	}

	log.Trace().Stringer("type", typ).Msg("sending message to runtime")

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

type messageBuilder func(header Header, bytes []byte) (Message, error)

var (
	builders = map[Type]messageBuilder{
		StartMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &StartMessage{
				Header:  header,
				Version: header.Flag.version(),
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		PollInputEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &PollInputEntry{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		OutputStreamEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &OutputStreamEntry{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		GetStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &GetStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		SetStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &SetStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		ClearStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &ClearStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		ClearAllStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &ClearAllStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		GetStateKeysEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &GetStateKeysEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		CompletionMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &CompletionMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		SleepEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &SleepEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		InvokeEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &InvokeEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
		BackgroundInvokeEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &BackgroundInvokeEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.Payload)
		},
	}
)

type StartMessage struct {
	Header
	Version uint16
	Payload protocol.StartMessage
}

type PollInputEntry struct {
	Header
	Payload protocol.PollInputStreamEntryMessage
}

type OutputStreamEntry struct {
	Header
	Payload protocol.OutputStreamEntryMessage
}

type GetStateEntryMessage struct {
	Header
	Payload protocol.GetStateEntryMessage
}

type SetStateEntryMessage struct {
	Header
	Payload protocol.SetStateEntryMessage
}

type ClearStateEntryMessage struct {
	Header
	Payload protocol.ClearStateEntryMessage
}

type ClearAllStateEntryMessage struct {
	Header
	Payload protocol.ClearAllStateEntryMessage
}

type GetStateKeysEntryMessage struct {
	Header
	Payload protocol.GetStateKeysEntryMessage
}

type CompletionMessage struct {
	Header
	Payload protocol.CompletionMessage
}

type SleepEntryMessage struct {
	Header
	Payload protocol.SleepEntryMessage
}

type InvokeEntryMessage struct {
	Header
	Payload protocol.InvokeEntryMessage
}

type BackgroundInvokeEntryMessage struct {
	Header
	Payload protocol.BackgroundInvokeEntryMessage
}
