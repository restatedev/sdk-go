// wire implements the grpc wire protocol that is used later on by the state machine
// to communicate with restate runtime.
package wire

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	protocol "github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

var (
	ErrUnexpectedMessage = fmt.Errorf("unexpected message")
)

const (
	// masks
	FlagCompleted   Flag = 0x0001
	FlagRequiresAck Flag = 0x8000

	VersionMask = 0x03FF
)
const (
	// control
	StartMessageType      Type = 0x0000
	CompletionMessageType Type = 0x0000 + 1
	SuspensionMessageType Type = 0x0000 + 2
	ErrorMessageType      Type = 0x0000 + 3
	EntryAckMessageType   Type = 0x0000 + 4
	EndMessageType        Type = 0x0000 + 5

	// Input/Output
	InputEntryMessageType  Type = 0x0400
	OutputEntryMessageType Type = 0x0400 + 1

	// State
	GetStateEntryMessageType      Type = 0x0800
	SetStateEntryMessageType      Type = 0x0800 + 1
	ClearStateEntryMessageType    Type = 0x0800 + 2
	ClearAllStateEntryMessageType Type = 0x0800 + 3
	GetStateKeysEntryMessageType  Type = 0x0800 + 4

	//SysCalls
	SleepEntryMessageType             Type = 0x0C00
	CallEntryMessageType              Type = 0x0C00 + 1
	OneWayCallEntryMessageType        Type = 0x0C00 + 2
	AwakeableEntryMessageType         Type = 0x0C00 + 3
	CompleteAwakeableEntryMessageType Type = 0x0C00 + 4
	RunEntryMessageType               Type = 0x0C00 + 5
)

type Type uint16

func (t Type) String() string {
	return fmt.Sprintf("0x%04X", uint16(t))
}

// Flag section of the header this can have
// a different meaning based on message type.
type Flag uint16

func (r Flag) Completed() bool {
	return r&FlagCompleted != 0
}

func (r Flag) Ack() bool {
	return r&FlagRequiresAck != 0
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
	proto.Message
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
	log    *zerolog.Logger
	stream io.ReadWriter
}

func NewProtocol(log *zerolog.Logger, stream io.ReadWriter) *Protocol {
	return &Protocol{log, stream}
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

	msg, err := builder(header, buf)
	if err != nil {
		return nil, err
	}

	s.log.Trace().Stringer("type", header.TypeCode).Interface("msg", msg).Msg("received message")
	return msg, nil
}

func (s *Protocol) Write(message Message, flag Flag) error {
	// all possible types sent by the sdk
	var typ Type
	switch message.(type) {
	case *StartMessage:
		typ = StartMessageType
	case *SuspensionMessage:
		typ = SuspensionMessageType
	case *InputEntryMessage:
		typ = InputEntryMessageType
	case *OutputEntryMessage:
		typ = OutputEntryMessageType
	case *ErrorMessage:
		typ = ErrorMessageType
	case *EndMessage:
		typ = EndMessageType
	case *GetStateEntryMessage:
		typ = GetStateEntryMessageType
	case *SetStateEntryMessage:
		typ = SetStateEntryMessageType
	case *ClearStateEntryMessage:
		typ = ClearStateEntryMessageType
	case *ClearAllStateEntryMessage:
		typ = ClearAllStateEntryMessageType
	case *GetStateKeysEntryMessage:
		typ = GetStateKeysEntryMessageType
	case *SleepEntryMessage:
		typ = SleepEntryMessageType
	case *CallEntryMessage:
		typ = CallEntryMessageType
	case *OneWayCallEntryMessage:
		typ = OneWayCallEntryMessageType
	case *AwakeableEntryMessage:
		typ = AwakeableEntryMessageType
	case *CompleteAwakeableEntryMessage:
		typ = CompleteAwakeableEntryMessageType
	case *RunEntryMessage:
		typ = RunEntryMessageType
	default:
		return fmt.Errorf("can not send message of unknown message type")
	}

	s.log.Trace().Stringer("type", typ).Interface("msg", message).Msg("sending message to runtime")

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
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.StartMessage)
		},
		EntryAckMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &EntryAckMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.EntryAckMessage)
		},
		InputEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &InputEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.InputEntryMessage)
		},
		OutputEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &OutputEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.OutputEntryMessage)
		},
		GetStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &GetStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.GetStateEntryMessage)
		},
		SetStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &SetStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.SetStateEntryMessage)
		},
		ClearStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &ClearStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.ClearStateEntryMessage)
		},
		ClearAllStateEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &ClearAllStateEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.ClearAllStateEntryMessage)
		},
		GetStateKeysEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &GetStateKeysEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.GetStateKeysEntryMessage)
		},
		CompletionMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &CompletionMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.CompletionMessage)
		},
		SleepEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &SleepEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.SleepEntryMessage)
		},
		CallEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &CallEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.CallEntryMessage)
		},
		OneWayCallEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &OneWayCallEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.OneWayCallEntryMessage)
		},
		AwakeableEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &AwakeableEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.AwakeableEntryMessage)
		},
		CompleteAwakeableEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &CompleteAwakeableEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.CompleteAwakeableEntryMessage)
		},
		RunEntryMessageType: func(header Header, bytes []byte) (Message, error) {
			msg := &RunEntryMessage{
				Header: header,
			}

			return msg, proto.Unmarshal(bytes, &msg.RunEntryMessage)
		},
	}
)

type StartMessage struct {
	Header
	protocol.StartMessage
}

type SuspensionMessage struct {
	Header
	protocol.SuspensionMessage
}

type InputEntryMessage struct {
	Header
	protocol.InputEntryMessage
}

type OutputEntryMessage struct {
	Header
	protocol.OutputEntryMessage
}

type ErrorMessage struct {
	Header
	protocol.ErrorMessage
}

type EndMessage struct {
	Header
	protocol.EndMessage
}

type GetStateEntryMessage struct {
	Header
	protocol.GetStateEntryMessage
}

type SetStateEntryMessage struct {
	Header
	protocol.SetStateEntryMessage
}

type ClearStateEntryMessage struct {
	Header
	protocol.ClearStateEntryMessage
}

type ClearAllStateEntryMessage struct {
	Header
	protocol.ClearAllStateEntryMessage
}

type GetStateKeysEntryMessage struct {
	Header
	protocol.GetStateKeysEntryMessage
}

type CompletionMessage struct {
	Header
	protocol.CompletionMessage
}

type SleepEntryMessage struct {
	Header
	protocol.SleepEntryMessage
}

type CallEntryMessage struct {
	Header
	protocol.CallEntryMessage
}

type OneWayCallEntryMessage struct {
	Header
	protocol.OneWayCallEntryMessage
}

type AwakeableEntryMessage struct {
	Header
	protocol.AwakeableEntryMessage
}

type CompleteAwakeableEntryMessage struct {
	Header
	protocol.CompleteAwakeableEntryMessage
}

type RunEntryMessage struct {
	Header
	protocol.RunEntryMessage
}

type EntryAckMessage struct {
	Header
	protocol.EntryAckMessage
}
