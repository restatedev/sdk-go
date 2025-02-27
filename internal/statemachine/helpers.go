package statemachine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

type loggerKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func getLogger(ctx context.Context) *slog.Logger {
	val, _ := ctx.Value(loggerKey{}).(*slog.Logger)
	return val
}

func TakeOutputAndWriteOut(ctx context.Context, machine *StateMachine, conn io.WriteCloser) error {
	buffer, err := machine.TakeOutput(ctx)
	if err == io.EOF {
		return conn.Close()
	} else if err != nil {
		return err
	}
	_, err = conn.Write(buffer)
	return err
}

func ConsumeOutput(ctx context.Context, machine *StateMachine, conn io.WriteCloser) error {
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

func ReadInputAndNotifyIt(ctx context.Context, tempBuf []byte, machine *StateMachine, conn io.Reader) error {
	for {
		read, err := conn.Read(tempBuf)
		if err == io.EOF {
			// Got EOF, notify and return back
			if err = machine.NotifyInputClosed(ctx); err != nil {
				return err
			}
		} else if err != nil {
			// Cannot read input anymore
			return fmt.Errorf("error when reading the input stream %e", err)
		}
		if read == 0 {
			return nil
		} else {
			if err = machine.NotifyInput(ctx, tempBuf[0:read]); err != nil {
				return err
			}
			if read < len(tempBuf) {
				// For now, nothing else to read here
				return nil
			}
		}
	}
}
