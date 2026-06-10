package restatecontext

import (
	"context"
	"io"
	"log/slog"
	"sync"

	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

var BufPool sync.Pool

func init() {
	BufPool = sync.Pool{New: func() interface{} {
		return make([]byte, 1024)
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
		read, err := restateCtx.conn.Read(tempBuf)
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
