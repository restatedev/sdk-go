package mocks

import (
	"reflect"
	"testing"
	"time"

	"github.com/restatedev/sdk-go/internal/restatecontext"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/converters"
	options "github.com/restatedev/sdk-go/internal/options"
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

		t := getT(_e.mock)
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

// MockRand is a helper method to mock a typical 'Rand' call on a ctx; return a mocked Rand object
func (_e *MockContext_Expecter) MockRand() *MockRand_Expecter {
	mockRand := NewMockRand(getT(_e.mock))
	_e.Rand().Once().Return(mockRand)
	return mockRand.EXPECT()
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
	restatecontext.Selectable
	mock.Mock
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

// MockRunAsyncFuture is a mock type for the RunAsyncFuture type
type MockRunAsyncFuture struct {
	restatecontext.Selectable
	mock.Mock
}

// ResponseAndReturn is a helper method to mock a typical 'Result' call on a RunAsyncFuture; return a concrete value or an error
func (_e *MockRunAsyncFuture_Expecter) ResultAndReturn(value any, err error) *MockRunAsyncFuture_Result_Call {
	return _e.Result(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockAttachFuture is a mock type for the AttachFuture type
type MockAttachFuture struct {
	restatecontext.Selectable
	mock.Mock
}

// ResponseAndReturn is a helper method to mock a typical 'Response' call on a AttachFuture; return a concrete value or an error
func (_e *MockAttachFuture_Expecter) ResponseAndReturn(value any, err error) *MockAttachFuture_Response_Call {
	return _e.Response(mock.AnythingOfType(pointerType(value))).RunAndReturn(func(i interface{}) error {
		if err != nil {
			return err
		}

		reflect.ValueOf(i).Elem().Set(reflect.ValueOf(value))
		return nil
	})
}

// MockAwakeableFuture is a mock type for the AwakeableFuture type
type MockAwakeableFuture struct {
	restatecontext.Selectable
	mock.Mock
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
	mockPromise := NewMockDurablePromise(getT(_e.mock))
	_e.Promise(promiseName).Once().Return(mockPromise)
	return mockPromise.EXPECT()
}

// MockDurablePromise is a mock type for the DurablePromise type
type MockDurablePromise struct {
	restatecontext.Selectable
	mock.Mock
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
	mockAfter := NewMockAfterFuture(getT(_e.mock))
	_e.After(duration).Once().Return(mockAfter)
	return mockAfter
}

// MockAfterFuture is a mock type for the AfterFuture type
type MockAfterFuture struct {
	restatecontext.Selectable
	mock.Mock
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

	mockSelector := NewMockSelector(getT(_e.mock))
	_e.Select(outFuts...).Once().Return(mockSelector)
	return mockSelector.EXPECT()
}
