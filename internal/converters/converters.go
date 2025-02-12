package converters

import (
	"github.com/restatedev/sdk-go/internal/futures"
	"github.com/restatedev/sdk-go/internal/state"
)

type ToInnerFuture interface {
	InnerFuture() futures.Selectable
}

type ResponseFuture[O any] struct {
	state.ResponseFuture
}

func (t ResponseFuture[O]) Response() (output O, err error) {
	err = t.ResponseFuture.Response(&output)
	return
}

func (t ResponseFuture[O]) InnerFuture() futures.Selectable {
	return t.ResponseFuture
}

type AwakeableFuture[T any] struct {
	state.AwakeableFuture
}

func (t AwakeableFuture[T]) Result() (output T, err error) {
	err = t.AwakeableFuture.Result(&output)
	return
}

func (t AwakeableFuture[T]) InnerFuture() futures.Selectable {
	return t.AwakeableFuture
}
