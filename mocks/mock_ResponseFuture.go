package mocks

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
	mock "github.com/stretchr/testify/mock"
)

// MockResponseFuture is a mock type for the ResponseFuture type
type MockResponseFuture struct {
	restatecontext.Selectable
	mock.Mock
}

type MockResponseFuture_Expecter struct {
	parent *MockResponseFuture
	mock   *mock.Mock
}

func (_m *MockResponseFuture) EXPECT() *MockResponseFuture_Expecter {
	return &MockResponseFuture_Expecter{mock: &_m.Mock}
}

// GetInvocationId provides a mock function with no fields
func (_m *MockResponseFuture) GetInvocationId() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetInvocationId")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockResponseFuture_GetInvocationId_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetInvocationId'
type MockResponseFuture_GetInvocationId_Call struct {
	*mock.Call
}

// GetInvocationId is a helper method to define mock.On call
func (_e *MockResponseFuture_Expecter) GetInvocationId() *MockResponseFuture_GetInvocationId_Call {
	return &MockResponseFuture_GetInvocationId_Call{Call: _e.mock.On("GetInvocationId")}
}

func (_c *MockResponseFuture_GetInvocationId_Call) Run(run func()) *MockResponseFuture_GetInvocationId_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockResponseFuture_GetInvocationId_Call) Return(_a0 string) *MockResponseFuture_GetInvocationId_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockResponseFuture_GetInvocationId_Call) RunAndReturn(run func() string) *MockResponseFuture_GetInvocationId_Call {
	_c.Call.Return(run)
	return _c
}

// Response provides a mock function with given fields: output
func (_m *MockResponseFuture) Response(output any) error {
	ret := _m.Called(output)

	if len(ret) == 0 {
		panic("no return value specified for Response")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(any) error); ok {
		r0 = rf(output)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockResponseFuture_Response_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Response'
type MockResponseFuture_Response_Call struct {
	*mock.Call
}

// Response is a helper method to define mock.On call
//   - output any
func (_e *MockResponseFuture_Expecter) Response(output interface{}) *MockResponseFuture_Response_Call {
	return &MockResponseFuture_Response_Call{Call: _e.mock.On("Response", output)}
}

func (_c *MockResponseFuture_Response_Call) Run(run func(output any)) *MockResponseFuture_Response_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockResponseFuture_Response_Call) Return(_a0 error) *MockResponseFuture_Response_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockResponseFuture_Response_Call) RunAndReturn(run func(any) error) *MockResponseFuture_Response_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockResponseFuture creates a new instance of MockResponseFuture. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockResponseFuture(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockResponseFuture {
	mock := &MockResponseFuture{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
