package restatecontext

import (
	"context"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"io"
	"log/slog"
	"sync"
)

var BufPool sync.Pool

func init() {
	BufPool = sync.Pool{New: func() interface{} {
		return make([]byte, 1024)
	}}
}

func takeOutputAndWriteOut(ctx context.Context, machine *statemachine.StateMachine, conn io.WriteCloser) error {
	buffer, err := machine.TakeOutput(ctx)
	if err == io.EOF {
		return conn.Close()
	} else if err != nil {
		return err
	}
	_, err = conn.Write(buffer)
	return err
}

func consumeOutput(ctx context.Context, machine *statemachine.StateMachine, conn io.WriteCloser) error {
	for {
		buffer, err := machine.TakeOutput(ctx)
		if err == io.EOF {
			return conn.Close()
		} else if err != nil {
			return err
		}

		_, err = conn.Write(buffer)
		if err != nil {
			return err
		}
	}
}

type readResult struct {
	nRead int
	buf   []byte
}

func (restateCtx *ctx) readInputLoop(logger *slog.Logger) {
	for {
		// Acquire buf
		tempBuf := BufPool.Get().([]byte)
		read, err := restateCtx.conn.Read(tempBuf)
		if err != nil {
			BufPool.Put(tempBuf)
			if err != io.EOF {
				logger.WarnContext(restateCtx, "Unexpected when reading input", log.Error(err))
			}
			close(restateCtx.readChan)
			return
		}
		if read != 0 {
			restateCtx.readChan <- readResult{
				nRead: read,
				buf:   tempBuf,
			}
		}
	}
}
