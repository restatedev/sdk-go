package mocks

import (
	"testing"

	options "github.com/restatedev/sdk-go/internal/options"
	state "github.com/restatedev/sdk-go/internal/state"
	mock "github.com/stretchr/testify/mock"
)

// MockClient is a mock type for the Client type
type MockClient struct {
	t *testing.T
	mock.Mock
}

type MockClient_Expecter struct {
	parent *MockClient
	mock   *mock.Mock
}

func (_m *MockClient) EXPECT() *MockClient_Expecter {
	return &MockClient_Expecter{parent: _m, mock: &_m.Mock}
}

// Request provides a mock function with given fields: input, output, opts
func (_m *MockClient) Request(input any, output any, opts ...options.RequestOption) error {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, input, output)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Request")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(any, any, ...options.RequestOption) error); ok {
		r0 = rf(input, output, opts...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClient_Request_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Request'
type MockClient_Request_Call struct {
	*mock.Call
}

// Request is a helper method to define mock.On call
//   - input any
//   - output any
//   - opts ...options.RequestOption
func (_e *MockClient_Expecter) Request(input interface{}, output interface{}, opts ...interface{}) *MockClient_Request_Call {
	return &MockClient_Request_Call{Call: _e.mock.On("Request",
		append([]interface{}{input, output}, opts...)...)}
}

func (_c *MockClient_Request_Call) Run(run func(input any, output any, opts ...options.RequestOption)) *MockClient_Request_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]options.RequestOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(options.RequestOption)
			}
		}
		run(args[0].(any), args[1].(any), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Request_Call) Return(_a0 error) *MockClient_Request_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Request_Call) RunAndReturn(run func(any, any, ...options.RequestOption) error) *MockClient_Request_Call {
	_c.Call.Return(run)
	return _c
}

// RequestFuture provides a mock function with given fields: input, opts
func (_m *MockClient) RequestFuture(input any, opts ...options.RequestOption) state.ResponseFuture {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, input)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for RequestFuture")
	}

	var r0 state.ResponseFuture
	if rf, ok := ret.Get(0).(func(any, ...options.RequestOption) state.ResponseFuture); ok {
		r0 = rf(input, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(state.ResponseFuture)
		}
	}

	return r0
}

// MockClient_RequestFuture_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RequestFuture'
type MockClient_RequestFuture_Call struct {
	*mock.Call
}

// RequestFuture is a helper method to define mock.On call
//   - input any
//   - opts ...options.RequestOption
func (_e *MockClient_Expecter) RequestFuture(input interface{}, opts ...interface{}) *MockClient_RequestFuture_Call {
	return &MockClient_RequestFuture_Call{Call: _e.mock.On("RequestFuture",
		append([]interface{}{input}, opts...)...)}
}

func (_c *MockClient_RequestFuture_Call) Run(run func(input any, opts ...options.RequestOption)) *MockClient_RequestFuture_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]options.RequestOption, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(options.RequestOption)
			}
		}
		run(args[0].(any), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_RequestFuture_Call) Return(_a0 state.ResponseFuture) *MockClient_RequestFuture_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_RequestFuture_Call) RunAndReturn(run func(any, ...options.RequestOption) state.ResponseFuture) *MockClient_RequestFuture_Call {
	_c.Call.Return(run)
	return _c
}

// Send provides a mock function with given fields: input, opts
func (_m *MockClient) Send(input any, opts ...options.SendOption) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, input)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockClient_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockClient_Send_Call struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - input any
//   - opts ...options.SendOption
func (_e *MockClient_Expecter) Send(input interface{}, opts ...interface{}) *MockClient_Send_Call {
	return &MockClient_Send_Call{Call: _e.mock.On("Send",
		append([]interface{}{input}, opts...)...)}
}

func (_c *MockClient_Send_Call) Run(run func(input any, opts ...options.SendOption)) *MockClient_Send_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]options.SendOption, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(options.SendOption)
			}
		}
		run(args[0].(any), variadicArgs...)
	})
	return _c
}

func (_c *MockClient_Send_Call) Return() *MockClient_Send_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockClient_Send_Call) RunAndReturn(run func(any, ...options.SendOption)) *MockClient_Send_Call {
	_c.Run(run)
	return _c
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClient(t *testing.T) *MockClient {
	mock := &MockClient{t: t}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
