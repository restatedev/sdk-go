package restatecontext

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/restatedev/sdk-go/internal/statemachine"
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
	err   error
}

func (restateCtx *ctx) readInputLoop() {
	defer func() {
		// make sure the channel is closed to avoid goroutine leak
		close(restateCtx.readChan)
	}()

	for {
		// Acquire buf
		tempBuf := BufPool.Get().([]byte)
		read, err := restateCtx.conn.Read(tempBuf)
		if err != nil {
			BufPool.Put(tempBuf)
			if err != io.EOF {
				err = fmt.Errorf("error when reading the input stream %e", err)
			}

			select {
			case restateCtx.readChan <- readResult{
				nRead: 0,
				buf:   nil,
				err:   err,
			}:
			case <-restateCtx.Done():
			}

			return
		}
		if read != 0 {

			select {
			case restateCtx.readChan <- readResult{
				nRead: read,
				buf:   tempBuf,
				err:   nil,
			}:
			case <-restateCtx.Done():
				return
			}

		}
	}
}
