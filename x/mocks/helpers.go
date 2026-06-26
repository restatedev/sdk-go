package mocks

import (
	rand2 "math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/restatedev/sdk-go/internal/restatecontext"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/genericfutures"
	options "github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/randsource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func pointerType(value any) string {
	return "*" + reflect.TypeOf(value).String()
}

func setT(t *testing.T, mock *mock.Mock) {
	mock.TestData().Set("t", t)
}

func getT(mock *mock.Mock) *testing.T {
	return mock.TestData().Get("t").MustInter().(*testing.T)
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockClient(t *testing.T) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)
	setT(t, &mock.Mock)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// NewMockContext creates a new instance of MockContext. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockContext(t *testing.T) *MockContext {
	mock := &MockContext{}
	mock.Mock.Test(t)
	setT(t, &mock.Mock)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// RunAndReturn is a helper method to mock a typical 'Run' call; return a concrete value or an error
func (_e *MockContext_Expecter) RunAndReturn(value any, err restate.TerminalError) *MockContext_Run_Call {
	return _e.Run(mock.Anything, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(f func(restatecontext.RunContext) (any, error), i any, ro ...options.RunOption) restate.TerminalError {
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
	return _e.Run(mock.Anything, mock.Anything).RunAndReturn(func(f func(restatecontext.RunContext) (any, error), i any, ro ...options.RunOption) restate.TerminalError {
		var value any
		var err error
		for {
			value, err = f(ctx)
			if err == nil || restate.IsTerminalError(err) {
				break
			}
			time.Sleep(time.Second)
		}

		t := getT(_e.mock)
		assert.Equal(t, expectedValue, value)
		assert.Equal(t, expectedErr, err)

		if err == nil {
			reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		}

		return restate.AsTerminalError(err)
	})
}

// GetAndReturn is a helper method to mock a typical 'Get' call; return a concrete value, or no value if nil interface is provided
func (_e *MockContext_Expecter) GetAndReturn(key interface{}, value any) *MockContext_Get_Call {
	return _e.Get(key, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(s string, i interface{}, g ...options.GetOption) (bool, restate.TerminalError) {
		if value == nil {
			return false, nil
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return true, nil
	})
}

// WithRandSeed makes the context's RandInstance, RandUUID and RandSource return
// deterministic values derived from the given seed, mirroring how the SDK seeds
// randomness from the invocation id at runtime. This is the way to control the
// randomness produced by the restate.Rand, restate.UUID and restate.RandSource
// public APIs in tests.
//
// All three are registered as optional, so a handler may use any subset of them.
// To force a specific UUID instead, use RandUUID().Return(...) directly.
func (_e *MockContext_Expecter) WithRandSeed(seed uint64) {
	source := randsource.NewFromSeed(seed)
	r := rand2.New(source.Copy())
	_e.RandInstance().Return(r).Maybe()
	_e.RandSource().Return(source).Maybe()
	_e.RandUUID().RunAndReturn(func() uuid.UUID { return randsource.UUIDFromRand(r) }).Maybe()
}

// MockServiceClient is a helper method to mock a typical 'Service' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockServiceClient(service, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(getT(_e.mock))
	_e.Service(service, method).Once().Return(mockClient)
	return mockClient.EXPECT()
}

// MockObjectClient is a helper method to mock a typical 'Object' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockObjectClient(service, key, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(getT(_e.mock))
	_e.Object(service, key, method).Once().Return(mockClient)
	return mockClient.EXPECT()
}

// MockWorkflowClient is a helper method to mock a typical 'Workflow' call on a ctx; return a mocked Client object
func (_e *MockContext_Expecter) MockWorkflowClient(service, workflowID, method interface{}) *MockClient_Expecter {
	mockClient := NewMockClient(getT(_e.mock))
	_e.Workflow(service, workflowID, method).Once().Return(mockClient)
	return mockClient.EXPECT()
}

// GetAndReturn is a helper method to mock a typical 'Request' call; return a concrete value or an error
func (_e *MockClient_Expecter) RequestAndReturn(input interface{}, value any, err restate.TerminalError) *MockClient_Request_Call {
	return _e.Request(input, mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i1, i2 interface{}, ro ...options.RequestOption) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i2).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockResponseFuture is a helper method to mock a typical 'RequestFuture' call on a client; return a mocked ResponseFuture object
func (_e *MockClient_Expecter) MockResponseFuture(input interface{}, opts ...interface{}) *MockResponseFuture {
	mockResponseFuture := NewMockResponseFuture(getT(_e.mock))

	_e.RequestFuture(input, opts...).Once().Return(mockResponseFuture)
	return mockResponseFuture
}

// MockResponseFuture is a helper method to mock a typical 'Send' call on a client; return a mocked Invocation object
func (_e *MockClient_Expecter) MockSend(input interface{}, opts ...interface{}) *MockInvocation {
	mockInvocation := NewMockInvocation(getT(_e.mock))

	_e.Send(input, opts...).Once().Return(mockInvocation)
	return mockInvocation
}

// MockResponseFuture is a mock type for the ResponseFuture type
type MockResponseFuture struct {
	restatecontext.Future
	mock.Mock
}

// ResponseAndReturn is a helper method to mock a typical 'Response' call on a ResponseFuture; return a concrete value or an error
func (_e *MockResponseFuture_Expecter) ResponseAndReturn(value any, err restate.TerminalError) *MockResponseFuture_Response_Call {
	return _e.Response(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockRunAsyncFuture is a mock type for the RunAsyncFuture type
type MockRunAsyncFuture struct {
	restatecontext.Future
	mock.Mock
}

// ResponseAndReturn is a helper method to mock a typical 'Result' call on a RunAsyncFuture; return a concrete value or an error
func (_e *MockRunAsyncFuture_Expecter) ResultAndReturn(value any, err restate.TerminalError) *MockRunAsyncFuture_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockAttachFuture is a mock type for the AttachFuture type
type MockAttachFuture struct {
	restatecontext.Future
	mock.Mock
}

// ResponseAndReturn is a helper method to mock a typical 'Response' call on a AttachFuture; return a concrete value or an error
func (_e *MockAttachFuture_Expecter) ResponseAndReturn(value any, err restate.TerminalError) *MockAttachFuture_Response_Call {
	return _e.Response(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockAwakeableFuture is a mock type for the AwakeableFuture type
type MockAwakeableFuture struct {
	restatecontext.Future
	mock.Mock
}

// ResultAndReturn is a helper method to mock a typical 'Result' call on a AwakeableFuture; return a concrete value or an error
func (_e *MockAwakeableFuture_Expecter) ResultAndReturn(value any, err restate.TerminalError) *MockAwakeableFuture_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

func (_e *MockContext_Expecter) PromiseByName(promiseName string) *MockDurablePromise_Expecter {
	mockPromise := NewMockDurablePromise(getT(_e.mock))
	_e.Promise(promiseName).Once().Return(mockPromise)
	return mockPromise.EXPECT()
}

// MockDurablePromise is a mock type for the DurablePromise type
type MockDurablePromise struct {
	restatecontext.Future
	mock.Mock
}

// PeekAndReturn is a helper method to mock a typical 'Peek' call on a DurablePromise; return a concrete value, no value, or an error
func (_e *MockDurablePromise_Expecter) PeekAndReturn(value any, ok bool, err restate.TerminalError) *MockDurablePromise_Peek_Call {
	return _e.Peek(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) (bool, restate.TerminalError) {
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
func (_e *MockDurablePromise_Expecter) ResultAndReturn(value any, err restate.TerminalError) *MockDurablePromise_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) restate.TerminalError {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockAfter is a helper method to mock a typical 'After' call on a ctx; return a mocked AfterFuture object
func (_e *MockContext_Expecter) MockAfter(duration interface{}) *MockAfterFuture {
	mockAfter := NewMockAfterFuture(getT(_e.mock))
	_e.After(duration).Once().Return(mockAfter)
	return mockAfter
}

// MockAfterFuture is a mock type for the AfterFuture type
type MockAfterFuture struct {
	restatecontext.Future
	mock.Mock
}

// MockWaitIter is a helper method to mock a typical WaitIter call on a ctx; return a mocked WaitIterator object
func (_e *MockContext_Expecter) MockWaitIter(futs ...interface{}) *MockWaitIterator_Expecter {
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
			if toInner, ok := actual.(genericfutures.ToFuture); ok {
				actual = toInner.Future()
			}

			return assert.ObjectsAreEqual(actual, expected)
		}))
	}

	mockWaitIter := NewMockWaitIterator(getT(_e.mock))
	_e.WaitIter(outFuts...).Once().Return(mockWaitIter)
	return mockWaitIter.EXPECT()
}
