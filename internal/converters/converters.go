package converters

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

type ToInnerFuture interface {
	InnerFuture() restatecontext.Selectable
}

type ResponseFuture[O any] struct {
	restatecontext.ResponseFuture
}

func (t ResponseFuture[O]) Response() (output O, err error) {
	err = t.ResponseFuture.Response(&output)
	return
}

func (t ResponseFuture[O]) InnerFuture() restatecontext.Selectable {
	return t.ResponseFuture
}

type AwakeableFuture[T any] struct {
	restatecontext.AwakeableFuture
}

func (t AwakeableFuture[T]) Result() (output T, err error) {
	err = t.AwakeableFuture.Result(&output)
	return
}

func (t AwakeableFuture[T]) InnerFuture() restatecontext.Selectable {
	return t.AwakeableFuture
}
