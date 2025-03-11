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
	return &MockResponseFuture_Expecter{parent: _m, mock: &_m.Mock}
}

// Response provides a mock function with given fields: output
func (_m *MockResponseFuture) Response(output interface{}) error {
	ret := _m.Called(output)

	if len(ret) == 0 {
		panic("no return value specified for Response")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(output)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (_m *MockResponseFuture) GetInvocationId() string {
	return ""
}

// MockResponseFuture_Response_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Response'
type MockResponseFuture_Response_Call struct {
	*mock.Call
}

// Response is a helper method to define mock.On call
//   - output interface{}
func (_e *MockResponseFuture_Expecter) Response(output interface{}) *MockResponseFuture_Response_Call {
	return &MockResponseFuture_Response_Call{Call: _e.mock.On("Response", output)}
}

func (_c *MockResponseFuture_Response_Call) Run(run func(output interface{})) *MockResponseFuture_Response_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *MockResponseFuture_Response_Call) Return(_a0 error) *MockResponseFuture_Response_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockResponseFuture_Response_Call) RunAndReturn(run func(interface{}) error) *MockResponseFuture_Response_Call {
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
