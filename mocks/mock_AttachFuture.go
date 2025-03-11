package mocks

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
	mock "github.com/stretchr/testify/mock"
)

// MockAttachFuture is a mock type for the AttachFuture type
type MockAttachFuture struct {
	restatecontext.Selectable
	mock.Mock
}

type MockAttachFuture_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAttachFuture) EXPECT() *MockAttachFuture_Expecter {
	return &MockAttachFuture_Expecter{mock: &_m.Mock}
}

// Response provides a mock function with given fields: output
func (_m *MockAttachFuture) Response(output any) error {
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

// MockAttachFuture_Response_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Response'
type MockAttachFuture_Response_Call struct {
	*mock.Call
}

// Response is a helper method to define mock.On call
//   - output any
func (_e *MockAttachFuture_Expecter) Response(output interface{}) *MockAttachFuture_Response_Call {
	return &MockAttachFuture_Response_Call{Call: _e.mock.On("Response", output)}
}

func (_c *MockAttachFuture_Response_Call) Run(run func(output any)) *MockAttachFuture_Response_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockAttachFuture_Response_Call) Return(_a0 error) *MockAttachFuture_Response_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAttachFuture_Response_Call) RunAndReturn(run func(any) error) *MockAttachFuture_Response_Call {
	_c.Call.Return(run)
	return _c
}

// handle provides a mock function with no fields
func (_m *MockAttachFuture) handle() uint32 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for handle")
	}

	var r0 uint32
	if rf, ok := ret.Get(0).(func() uint32); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint32)
	}

	return r0
}

// MockAttachFuture_handle_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'handle'
type MockAttachFuture_handle_Call struct {
	*mock.Call
}

// handle is a helper method to define mock.On call
func (_e *MockAttachFuture_Expecter) handle() *MockAttachFuture_handle_Call {
	return &MockAttachFuture_handle_Call{Call: _e.mock.On("handle")}
}

func (_c *MockAttachFuture_handle_Call) Run(run func()) *MockAttachFuture_handle_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockAttachFuture_handle_Call) Return(_a0 uint32) *MockAttachFuture_handle_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAttachFuture_handle_Call) RunAndReturn(run func() uint32) *MockAttachFuture_handle_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAttachFuture creates a new instance of MockAttachFuture. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAttachFuture(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAttachFuture {
	mock := &MockAttachFuture{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
