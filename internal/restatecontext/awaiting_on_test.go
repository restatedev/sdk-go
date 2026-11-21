package restatecontext

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

// These tests run handlers through ExecuteInvocation with the embedded shared core, speaking
// service protocol V7 over an in-memory stream, and check when the SDK writes AwaitingOnMessage.

// Service protocol message types, see service-protocol/dev/restate/service/protocol.proto.
const (
	msgStart                   uint16 = 0x0000
	msgSuspension              uint16 = 0x0001
	msgError                   uint16 = 0x0002
	msgProposeRunCompletion    uint16 = 0x0005
	msgAwaitingOn              uint16 = 0x0006
	msgProposeRunCompletionAck uint16 = 0x0007
	msgInputCommand            uint16 = 0x0400
	msgSleepCompletion         uint16 = 0x800C
	msgCallCompletion          uint16 = 0x800D
)

// The first signal id available to awakeables; ids up to 16 are reserved for built-in signals.
const firstAwakeableSignalID = 17

// waitTimeout only bounds how long a broken test hangs; the tests never wait for time to pass.
const waitTimeout = 10 * time.Second

func TestAwaitingOnOncePerInputBatch(t *testing.T) {
	inv := startInvocation(t, func(ctx Context) {
		// The call's result notification (completion id 2) is the unrelated input sent below.
		_ = ctx.Service("Svc", "method").RequestFuture(nil)
		var out string
		_ = ctx.Awakeable().Result(&out)
	})

	// Waiting on the awakeable, the SDK must flush an AwaitingOn before blocking for input: no input
	// is sent until it shows up.
	frames := inv.waitForCount(msgAwaitingOn, 1)
	awaiting := decodeAwaitingOn(t, lastOfType(frames, msgAwaitingOn))
	require.False(t, awaiting.executingSideEffects)
	require.Contains(t, awaiting.signals, uint32(firstAwakeableSignalID))

	// A notification for something the handler isn't awaiting, bigger than the 1 KiB the SDK used
	// to read per chunk. The SDK may re-send AwaitingOn once after processing it, not once per chunk.
	inv.send(callCompletion(2, bytes.Repeat([]byte("a"), 20_000)))
	inv.waitForCount(msgAwaitingOn, 2)

	inv.closeInput()
	frames = inv.waitDone()
	require.Equal(t, 2, countOfType(frames, msgAwaitingOn), "AwaitingOn messages written: %v", frameTypes(frames))
	requireSuspendedCleanly(t, frames)
}

func TestAwaitingOnOncePerDrainedInputBatch(t *testing.T) {
	startAwaiting := make(chan struct{})
	inv := startInvocation(t, func(ctx Context) {
		// The call's result notification (completion id 2) is the unrelated input sent below.
		_ = ctx.Service("Svc", "method").RequestFuture(nil)
		<-startAwaiting
		var out string
		_ = ctx.Awakeable().Result(&out)
	})
	startHandler := sync.OnceFunc(func() { close(startAwaiting) })
	t.Cleanup(startHandler) // so that a failing test still lets the handler end

	// While the handler doesn't await anything yet, the SDK reads ahead: a notification of
	// inputReadAhead full chunks (plus its header, read separately) is read in full, whatever
	// is not buffered in readChan being held by the input reader.
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		inv.send(callCompletion(2, bytes.Repeat([]byte("a"), inputReadAhead*inputBufferSize)))
	}()
	waitClosed(t, sent, "the SDK to read ahead the whole notification")
	require.Zero(t, countOfType(inv.output.frames(), msgAwaitingOn))

	// Once the handler awaits, everything already read must be handed to the core before asking it
	// for progress: one AwaitingOn for the batch, possibly one more for the chunk the reader held,
	// rather than one per chunk.
	startHandler()
	inv.waitForCount(msgAwaitingOn, 1)
	inv.closeInput()
	frames := inv.waitDone()
	require.LessOrEqual(t, countOfType(frames, msgAwaitingOn), 2, "messages written: %v", frameTypes(frames))
	requireSuspendedCleanly(t, frames)
}

func TestAwaitingOnNotSentWhileRunExecutes(t *testing.T) {
	runStarted := make(chan struct{})
	releaseRun := make(chan struct{})
	sleepDone := make(chan struct{})
	inv := startInvocation(t, func(ctx Context) {
		run := ctx.RunAsync(func(RunContext) (any, error) { // completion id 1
			close(runStarted)
			<-releaseRun
			return "result", nil
		})
		// The core only executes a run that is awaited. The sleep (completion id 2) wins, so the
		// handler then waits on an awakeable while the run is still executing.
		ctx.WaitIter(run, ctx.After(time.Hour)).Next()
		close(sleepDone)
		var out string
		_ = ctx.Awakeable().Result(&out)
	})

	release := sync.OnceFunc(func() { close(releaseRun) })
	t.Cleanup(release) // so that a failing test still lets the handler end
	waitClosed(t, runStarted, "run to start")
	inv.send(sleepCompletion(2))
	waitClosed(t, sleepDone, "sleep to complete")

	// Once the run's result is proposed nothing executes anymore, so the SDK sends AwaitingOn, and
	// sends a fresh one after the runtime acknowledges the proposal.
	release()
	inv.waitForCount(msgAwaitingOn, 1)
	inv.send(proposeRunCompletionAck(1))
	inv.waitForCount(msgAwaitingOn, 2)

	inv.closeInput()
	frames := inv.waitDone()

	// No AwaitingOn may be written while the run executes, that is before its proposal.
	var sequence []uint16
	for _, f := range frames {
		if f.ty == msgProposeRunCompletion || f.ty == msgAwaitingOn {
			sequence = append(sequence, f.ty)
		}
	}
	require.Equal(t, []uint16{msgProposeRunCompletion, msgAwaitingOn, msgAwaitingOn}, sequence,
		"messages written: %v", frameTypes(frames))
	for _, f := range frames {
		if f.ty == msgAwaitingOn {
			require.False(t, decodeAwaitingOn(t, f).executingSideEffects)
		}
	}
	requireSuspendedCleanly(t, frames)
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(waitTimeout):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func requireSuspendedCleanly(t *testing.T, frames []frame) {
	t.Helper()
	require.Zero(t, countOfType(frames, msgError), "messages written: %v", frameTypes(frames))
	require.NotEmpty(t, frames)
	require.Equal(t, msgSuspension, frames[len(frames)-1].ty, "messages written: %v", frameTypes(frames))
}

// --- Invocation harness

type testInvocation struct {
	t      *testing.T
	input  *io.PipeWriter
	output *recordingWriter
	done   chan struct{}
}

// startInvocation runs fn as the handler of a fresh invocation, with its input stream open.
func startInvocation(t *testing.T, fn func(ctx Context)) *testInvocation {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	core, err := statemachine.NewCore(ctx)
	require.NoError(t, err)
	contentType := pbinternal.Header{}
	contentType.SetKey("content-type")
	contentType.SetValue("application/vnd.restate.invocation.v7")
	stateMachine, err := core.NewStateMachine(ctx, []*pbinternal.Header{&contentType})
	require.NoError(t, err)

	// What the server's request handler does before ExecuteInvocation: feed the known entries.
	require.NoError(t, stateMachine.NotifyInput(ctx, append(startMessage(), inputCommand()...)))
	ready, err := stateMachine.IsReadyToExecute(ctx)
	require.NoError(t, err)
	require.True(t, ready)

	inputReader, inputWriter := io.Pipe()
	inv := &testInvocation{t: t, input: inputWriter, output: newRecordingWriter(), done: make(chan struct{})}
	stream := struct {
		io.Reader
		io.Writer
	}{inputReader, inv.output}
	logger := slog.New(slog.DiscardHandler)
	go func() {
		defer close(inv.done)
		_ = ExecuteInvocation(ctx, logger, stateMachine, stream, testHandler(fn), false, logger.Handler(), nil)
	}()

	t.Cleanup(func() {
		_ = inputWriter.Close()
		select {
		case <-inv.done:
			// The state machine is only safe to free once the handler is done with it.
			_ = stateMachine.Free(ctx)
			_ = core.Close(ctx)
		case <-time.After(waitTimeout):
			t.Error("handler did not return")
		}
		cancel() // stops the SDK's input reader
	})
	return inv
}

// send writes to the invocation's input. It returns once the SDK has read all of it.
func (inv *testInvocation) send(messages ...[]byte) {
	inv.t.Helper()
	_, err := inv.input.Write(bytes.Join(messages, nil))
	require.NoError(inv.t, err)
}

func (inv *testInvocation) closeInput() {
	require.NoError(inv.t, inv.input.Close())
}

// waitForCount waits until the SDK has written at least n messages of type ty.
func (inv *testInvocation) waitForCount(ty uint16, n int) []frame {
	inv.t.Helper()
	return inv.output.waitFor(inv.t, func(frames []frame) bool { return countOfType(frames, ty) >= n })
}

// waitDone waits for the handler to return and gives everything the SDK wrote.
func (inv *testInvocation) waitDone() []frame {
	inv.t.Helper()
	select {
	case <-inv.done:
	case <-time.After(waitTimeout):
		inv.t.Fatalf("handler did not return; messages written: %v", frameTypes(inv.output.frames()))
	}
	return inv.output.frames()
}

type testHandler func(ctx Context)

func (h testHandler) GetOptions() *options.HandlerOptions       { return &options.HandlerOptions{} }
func (h testHandler) InputPayload() *encoding.InputPayload      { return nil }
func (h testHandler) OutputPayload() *encoding.OutputPayload    { return nil }
func (h testHandler) HandlerType() *internal.ServiceHandlerType { return nil }
func (h testHandler) Call(ctx Context, _ []byte) ([]byte, error) {
	h(ctx)
	return nil, nil
}

// recordingWriter records what the SDK writes and wakes up anyone waiting for it.
type recordingWriter struct {
	mu      sync.Mutex
	written []byte
	changed chan struct{} // closed and replaced on every write
}

func newRecordingWriter() *recordingWriter {
	return &recordingWriter{changed: make(chan struct{})}
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.written = append(w.written, p...)
	close(w.changed)
	w.changed = make(chan struct{})
	return len(p), nil
}

func (w *recordingWriter) frames() []frame {
	w.mu.Lock()
	defer w.mu.Unlock()
	return parseFrames(w.written)
}

func (w *recordingWriter) waitFor(t *testing.T, condition func([]frame) bool) []frame {
	t.Helper()
	deadline := time.After(waitTimeout)
	for {
		w.mu.Lock()
		frames, changed := parseFrames(w.written), w.changed
		w.mu.Unlock()
		if condition(frames) {
			return frames
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatalf("timed out; messages written: %v", frameTypes(frames))
		}
	}
}

// --- Protocol framing: a 64-bit header (type: 16 bits, flags: 16 bits, length: 32 bits), then the body.

type frame struct {
	ty   uint16
	body []byte
}

func encodeFrame(ty uint16, body []byte) []byte {
	return append(binary.BigEndian.AppendUint64(nil, uint64(ty)<<48|uint64(len(body))), body...)
}

func parseFrames(b []byte) []frame {
	var frames []frame
	for len(b) >= 8 {
		header := binary.BigEndian.Uint64(b)
		length := int(uint32(header))
		if len(b) < 8+length {
			break
		}
		frames = append(frames, frame{ty: uint16(header >> 48), body: b[8 : 8+length]})
		b = b[8+length:]
	}
	return frames
}

func countOfType(frames []frame, ty uint16) int {
	n := 0
	for _, f := range frames {
		if f.ty == ty {
			n++
		}
	}
	return n
}

func lastOfType(frames []frame, ty uint16) frame {
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].ty == ty {
			return frames[i]
		}
	}
	panic("no such frame")
}

func frameTypes(frames []frame) []string {
	names := map[uint16]string{
		msgSuspension: "Suspension", msgError: "Error", 0x0003: "End",
		msgProposeRunCompletion: "ProposeRunCompletion", msgAwaitingOn: "AwaitingOn",
	}
	types := make([]string, len(frames))
	for i, f := range frames {
		if name, ok := names[f.ty]; ok {
			types[i] = name
		} else {
			types[i] = fmt.Sprintf("0x%04X", f.ty)
		}
	}
	return types
}

// --- Protocol messages, encoded by hand: the SDK has no Go types for them.

func startMessage() []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType) // id
	b = protowire.AppendBytes(b, []byte("0123456789abcdef"))
	b = protowire.AppendTag(b, 2, protowire.BytesType) // debug_id
	b = protowire.AppendString(b, "inv_test")
	b = protowire.AppendTag(b, 3, protowire.VarintType) // known_entries: the input command
	b = protowire.AppendVarint(b, 1)
	return encodeFrame(msgStart, b)
}

func inputCommand() []byte {
	var b []byte
	b = protowire.AppendTag(b, 14, protowire.BytesType) // value
	b = protowire.AppendBytes(b, value([]byte("{}")))
	return encodeFrame(msgInputCommand, b)
}

func value(content []byte) []byte {
	b := protowire.AppendTag(nil, 1, protowire.BytesType) // content
	return protowire.AppendBytes(b, content)
}

func sleepCompletion(completionID uint32) []byte {
	b := protowire.AppendTag(nil, 1, protowire.VarintType) // completion_id
	b = protowire.AppendVarint(b, uint64(completionID))
	b = protowire.AppendTag(b, 4, protowire.BytesType) // void
	b = protowire.AppendBytes(b, nil)
	return encodeFrame(msgSleepCompletion, b)
}

func callCompletion(completionID uint32, content []byte) []byte {
	b := protowire.AppendTag(nil, 1, protowire.VarintType) // completion_id
	b = protowire.AppendVarint(b, uint64(completionID))
	b = protowire.AppendTag(b, 5, protowire.BytesType) // value
	b = protowire.AppendBytes(b, value(content))
	return encodeFrame(msgCallCompletion, b)
}

func proposeRunCompletionAck(completionID uint32) []byte {
	b := protowire.AppendTag(nil, 1, protowire.VarintType) // completion_id
	b = protowire.AppendVarint(b, uint64(completionID))
	return encodeFrame(msgProposeRunCompletionAck, b)
}

type awaitingOn struct {
	completions, signals []uint32 // of the whole future tree
	executingSideEffects bool
}

func decodeAwaitingOn(t *testing.T, f frame) awaitingOn {
	t.Helper()
	require.Equal(t, msgAwaitingOn, f.ty)
	var result awaitingOn
	// AwaitingOnMessage: Future awaiting_on = 1; bool executing_side_effects = 2.
	// Future: repeated uint32 waiting_completions = 1 (packed); repeated uint32 waiting_signals = 2
	// (packed); repeated Future nested_futures = 4. Other fields are skipped.
	var decode func(b []byte, inFuture bool)
	decode = func(b []byte, inFuture bool) {
		for len(b) > 0 {
			num, typ, n := protowire.ConsumeTag(b)
			require.GreaterOrEqual(t, n, 0, "malformed AwaitingOnMessage")
			b = b[n:]
			switch {
			case !inFuture && num == 1, inFuture && num == 4:
				v, m := protowire.ConsumeBytes(b)
				require.GreaterOrEqual(t, m, 0, "malformed AwaitingOnMessage")
				decode(v, true)
				b = b[m:]
			case !inFuture && num == 2:
				v, m := protowire.ConsumeVarint(b)
				require.GreaterOrEqual(t, m, 0, "malformed AwaitingOnMessage")
				result.executingSideEffects = v != 0
				b = b[m:]
			case inFuture && (num == 1 || num == 2) && typ == protowire.BytesType: // packed
				packed, m := protowire.ConsumeBytes(b)
				require.GreaterOrEqual(t, m, 0, "malformed AwaitingOnMessage")
				for len(packed) > 0 {
					id, k := protowire.ConsumeVarint(packed)
					require.GreaterOrEqual(t, k, 0, "malformed AwaitingOnMessage")
					if num == 1 {
						result.completions = append(result.completions, uint32(id))
					} else {
						result.signals = append(result.signals, uint32(id))
					}
					packed = packed[k:]
				}
				b = b[m:]
			default:
				m := protowire.ConsumeFieldValue(num, typ, b)
				require.GreaterOrEqual(t, m, 0, "malformed AwaitingOnMessage")
				b = b[m:]
			}
		}
	}
	decode(f.body, false)
	return result
}
