package mocks

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
	"reflect"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/converters"
	options "github.com/restatedev/sdk-go/internal/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func pointerType(value any) string {
	return "*" + reflect.TypeOf(value).String()
}

// RunAndReturn is a helper method to mock a typical 'Run' call; return a concrete value or an error
func (_e *MockContext_Expecter) RunAndReturn(value any, err error) *MockContext_Run_Call {
	return _e.Run(mock.Anything, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(f func(restatecontext.RunContext) (any, error), i any, ro ...options.RunOption) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// RunAndExpect is a helper method to mock a 'Run' call where you want to execute the function provided to Run. Non terminal errors will be retried
// indefinitely, subject to a 1 second delay between retries. The final result or terminal error will be compared to the provided values.
func (_e *MockContext_Expecter) RunAndExpect(ctx restatecontext.RunContext, expectedValue any, expectedErr error) *MockContext_Run_Call {
	return _e.Run(mock.Anything, mock.Anything).RunAndReturn(func(f func(restatecontext.RunContext) (any, error), i any, ro ...options.RunOption) error {
		var value any
		var err error
		for {
			value, err = f(ctx)
			if err == nil || restate.IsTerminalError(err) {
				break
			}
			time.Sleep(time.Second)
		}

		assert.Equal(_e.parent.t, expectedValue, value)
		assert.Equal(_e.parent.t, expectedErr, err)

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

// MockRand is a helper method to mock a typical 'Rand' call on a ctx; return a mocked Rand object
func (_e *MockContext_Expecter) MockRand() *MockRand_Expecter {
	mockRand := NewMockRand(_e.parent.t)
	_e.Rand().Once().Return(mockRand)
	return mockRand.EXPECT()
}

// MockServiceClient is a helper method to mock a typical 'Service' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockServiceClient(service, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(_e.parent.t)
	_e.Service(service, method).Once().Return(mockClient)
	return mockClient.EXPECT()
}

// MockObjectClient is a helper method to mock a typical 'Object' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockObjectClient(service, key, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(_e.parent.t)
	_e.Object(service, key, method).Once().Return(mockClient)
	return mockClient.EXPECT()
}

// MockWorkflowClient is a helper method to mock a typical 'Workflow' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockWorkflowClient(service, workflowID, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(_e.parent.t)
	_e.Workflow(service, workflowID, method).Once().Return(mockClient)
	return mockClient.EXPECT()
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

// MockResponseFuture is a helper method to mock a typical 'RequestFuture' call on a client; return a mocked ResponseFuture object
func (_e *MockClient_Expecter) MockResponseFuture(input interface{}, opts ...interface{}) *MockResponseFuture {
	mockResponseFuture := NewMockResponseFuture(_e.parent.t)

	_e.RequestFuture(input, opts...).Once().Return(mockResponseFuture)
	return mockResponseFuture
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

func (_e *MockContext_Expecter) PromiseByName(promiseName string) *MockDurablePromise_Expecter {
	mockPromise := NewMockDurablePromise(_e.parent.t)
	_e.Promise(promiseName).Once().Return(mockPromise)
	return mockPromise.EXPECT()
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

// MockAfter is a helper method to mock a typical 'After' call on a ctx; return a mocked AfterFuture object
func (_e *MockContext_Expecter) MockAfter(duration interface{}) *MockAfterFuture {
	mockAfter := NewMockAfterFuture(_e.parent.t)
	_e.After(duration).Once().Return(mockAfter)
	return mockAfter
}

// MockSelector is a helper method to mock a typical 'Select' call on a ctx; return a mocked Selector object
func (_e *MockContext_Expecter) MockSelector(futs ...interface{}) *MockSelector_Expecter {
	outFuts := make([]interface{}, 0, len(futs))

	for _, expected := range futs {
		expected := expected

		if assert.ObjectsAreEqual(expected, mock.Anything) {
			outFuts = append(outFuts, expected)
			continue
		}

		outFuts = append(outFuts, mock.MatchedBy(func(actual interface{}) bool {
			if assert.ObjectsAreEqual(actual, mock.Anything) {
				return true
			}

			// support the case where the future is wrapped in some converter
			if toInner, ok := actual.(converters.ToInnerFuture); ok {
				actual = toInner.InnerFuture()
			}

			return assert.ObjectsAreEqual(actual, expected)
		}))
	}

	mockSelector := NewMockSelector(_e.parent.t)
	_e.Select(outFuts...).Once().Return(mockSelector)
	return mockSelector.EXPECT()
}
