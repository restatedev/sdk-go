package restatecontext

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

// inputBufferSize is the size of the buffers used to read the invocation input stream.
//
// Every chunk read is handed to the state machine with NotifyInput, and pollProgress then calls
// DoProgress, which re-sends AwaitingOnMessage while the invocation keeps waiting. Small buffers
// split a single notification into many chunks (a 20 KB one used to take 20 reads at 1 KiB, each
// followed by a redundant AwaitingOnMessage), so the buffer should fit typical messages whole.
// 32 KiB holds two default-sized (16 KiB) HTTP/2 DATA frames and matches io.Copy's buffer size,
// while staying small next to the per-invocation WASM instance (192 KiB of linear memory when
// fresh): readInputLoop pins one buffer for as long as it waits for input. Buffers go back to
// BufPool once their bytes have been copied into the state machine.
const inputBufferSize = 32 * 1024

// inputReadAhead is how many chunks readInputLoop may read ahead of pollProgress, as the buffer of
// readChan.
//
// While pollProgress feeds a chunk to the state machine, readInputLoop keeps reading, so that the
// next pollProgress iteration can drain several chunks at once (see drainAvailableInput) and send
// one AwaitingOnMessage for all of them, rather than one every other chunk as with an unbuffered
// channel. Read ahead chunks only exist while input arrives faster than it's processed: each slot
// can then hold one more inputBufferSize buffer for the invocation.
const inputReadAhead = 4

var BufPool sync.Pool

func init() {
	BufPool = sync.Pool{New: func() interface{} {
		return make([]byte, inputBufferSize)
	}}
}

func takeOutputAndWriteOut(ctx context.Context, machine *statemachine.StateMachine, conn io.Writer) error {
	buffer, err := machine.TakeOutput(ctx)
	if err != nil {
		return err
	}
	_, err = conn.Write(buffer)
	return err
}

type readResult struct {
	nRead int
	buf   []byte
}

func (restateCtx *ctx) readInputLoop(logger *slog.Logger) {
	defer close(restateCtx.readChan)

	for {
		tempBuf := BufPool.Get().([]byte)
		read, err := restateCtx.stream.Read(tempBuf)
		if err != nil {
			BufPool.Put(tempBuf)
			if err != io.EOF {
				logger.WarnContext(restateCtx, "Unexpected when reading input", log.Error(err))
			}
			return
		}
		if read != 0 {
			select {
			case restateCtx.readChan <- readResult{nRead: read, buf: tempBuf}:
			case <-restateCtx.Done():
				BufPool.Put(tempBuf)
				return
			}
		}
	}
}
