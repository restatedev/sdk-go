package mocks

import (
	"reflect"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	options "github.com/restatedev/sdk-go/internal/options"
	state "github.com/restatedev/sdk-go/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func pointerType(value any) string {
	return "*" + reflect.TypeOf(value).String()
}

// RunAndReturn is a helper method to mock a typical 'Run' call; return a concrete value or an error
func (_e *MockContext_Expecter) RunAndReturn(value any, err error) *MockContext_Run_Call {
	return _e.Run(mock.Anything, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(f func(state.RunContext) (any, error), i any, ro ...options.RunOption) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// RunAndExpect is a helper method to mock a 'Run' call where you want to execute the function provided to Run. Non terminal errors will be retried
// indefinitely, subject to a 1 second delay between retries. The final result or terminal error will be compared to the provided values.
func (_e *MockContext_Expecter) RunAndExpect(t *testing.T, ctx state.RunContext, expectedValue any, expectedErr error) *MockContext_Run_Call {
	return _e.Run(mock.Anything, mock.Anything).RunAndReturn(func(f func(state.RunContext) (any, error), i any, ro ...options.RunOption) error {
		var value any
		var err error
		for {
			value, err = f(ctx)
			if err == nil || restate.IsTerminalError(err) {
				break
			}
			time.Sleep(time.Second)
		}

		assert.Equal(t, expectedValue, value)
		assert.Equal(t, expectedErr, err)

		if err == nil {
			reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		}

		return err
	})
}

// GetAndReturn is a helper method to mock a typical 'Get' call; return a concrete value, or no value if nil interface is provided
func (_e *MockContext_Expecter) GetAndReturn(key interface{}, value any) *MockContext_Get_Call {
	return _e.Get(key, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(s string, i interface{}, g ...options.GetOption) (bool, error) {
		if value == nil {
			return false, nil
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return true, nil
	})
}

// GetAndReturn is a helper method to mock a typical 'Request' call; return a concrete value or an error
func (_e *MockClient_Expecter) RequestAndReturn(input interface{}, value any, err error) *MockClient_Request_Call {
	return _e.Request(input, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i1, i2 interface{}, ro ...options.RequestOption) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i2).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// ResponseAndReturn is a helper method to mock a typical 'Response' call on a ResponseFuture; return a concrete value or an error
func (_e *MockResponseFuture_Expecter) ResponseAndReturn(value any, err error) *MockResponseFuture_Response_Call {
	return _e.Response(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// ResultAndReturn is a helper method to mock a typical 'Result' call on a AwakeableFuture; return a concrete value or an error
func (_e *MockAwakeableFuture_Expecter) ResultAndReturn(value any, err error) *MockAwakeableFuture_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// PeekAndReturn is a helper method to mock a typical 'Peek' call on a DurablePromise; return a concrete value, no value, or an error
func (_e *MockDurablePromise_Expecter) PeekAndReturn(value any, ok bool, err error) *MockDurablePromise_Peek_Call {
	return _e.Peek(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) (bool, error) {
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return true, nil
	})
}

// ResultAndReturn is a helper method to mock a typical 'Result' call on a DurablePromise; return a concrete value or an error
func (_e *MockDurablePromise_Expecter) ResultAndReturn(value any, err error) *MockDurablePromise_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}
