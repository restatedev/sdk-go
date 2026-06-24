package genericfutures

import (
	"github.com/restatedev/sdk-go/internal/errors"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

type ToFuture interface {
	Future() restatecontext.Future
}

type ResponseFuture[O any] struct {
	restatecontext.ResponseFuture
}

func (t ResponseFuture[O]) Response() (output O, err errors.TerminalError) {
	err = t.ResponseFuture.Response(&output)
	return
}

func (t ResponseFuture[O]) Future() restatecontext.Future {
	return t.ResponseFuture
}

type RunAsyncFuture[O any] struct {
	restatecontext.RunAsyncFuture
}

func (t RunAsyncFuture[O]) Result() (output O, err errors.TerminalError) {
	err = t.RunAsyncFuture.Result(&output)
	return
}

func (t RunAsyncFuture[O]) Future() restatecontext.Future {
	return t.RunAsyncFuture
}

type AttachFuture[O any] struct {
	restatecontext.AttachFuture
}

func (t AttachFuture[O]) Response() (output O, err errors.TerminalError) {
	err = t.AttachFuture.Response(&output)
	return
}

func (t AttachFuture[O]) Future() restatecontext.Future {
	return t.AttachFuture
}

type AwakeableFuture[T any] struct {
	restatecontext.AwakeableFuture
}

func (t AwakeableFuture[T]) Result() (output T, err errors.TerminalError) {
	err = t.AwakeableFuture.Result(&output)
	return
}

func (t AwakeableFuture[T]) Future() restatecontext.Future {
	return t.AwakeableFuture
}

type SignalFuture[T any] struct {
	restatecontext.SignalFuture
}

func (t SignalFuture[T]) Result() (output T, err errors.TerminalError) {
	err = t.SignalFuture.Result(&output)
	return
}

func (t SignalFuture[T]) Future() restatecontext.Future {
	return t.SignalFuture
}
