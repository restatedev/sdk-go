package restatecontext

import (
	_ "embed"
	"fmt"
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/statemachine"
)

func (restateCtx *ctx) Set(key string, value any, opts ...options.SetOption) {
	o := options.SetOptions{}
	for _, opt := range opts {
		opt.BeforeSet(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	bytes, err := encoding.Marshal(o.Codec, value)
	if err != nil {
		panic(fmt.Errorf("failed to marshal Set value: %w", err))
	}

	err = restateCtx.stateMachine.SysStateSet(restateCtx, key, bytes)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}

func (restateCtx *ctx) Clear(key string) {
	err := restateCtx.stateMachine.SysStateClear(restateCtx, key)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}

// ClearAll drops all associated keys
func (restateCtx *ctx) ClearAll() {
	err := restateCtx.stateMachine.SysStateClearAll(restateCtx)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()
}

func (restateCtx *ctx) Get(key string, output any, opts ...options.GetOption) (bool, error) {
	o := options.GetOptions{}
	for _, opt := range opts {
		opt.BeforeGet(&o)
	}
	if o.Codec == nil {
		o.Codec = encoding.JSONCodec
	}

	handle, err := restateCtx.stateMachine.SysStateGet(restateCtx, key)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	ar := newAsyncResult(restateCtx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueVoid:
		return false, nil
	case statemachine.ValueSuccess:
		{
			if err := encoding.Unmarshal(o.Codec, result.Success, output); err != nil {
				panic(fmt.Errorf("failed to unmarshal Get state into output: %w", err))
			}
			return true, err
		}
	case statemachine.ValueFailure:
		return true, errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))

	}
}

func (restateCtx *ctx) Keys() ([]string, error) {
	handle, err := restateCtx.stateMachine.SysStateGetKeys(restateCtx)
	if err != nil {
		panic(err)
	}
	restateCtx.checkStateTransition()

	ar := newAsyncResult(restateCtx, handle)
	switch result := ar.pollProgressAndLoadValue().(type) {
	case statemachine.ValueStateKeys:
		return result.Keys, nil
	case statemachine.ValueFailure:
		return nil, errorFromFailure(result)
	default:
		panic(fmt.Errorf("unexpected value %s", result))
	}
}
