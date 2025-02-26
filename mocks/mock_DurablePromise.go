package mocks

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
	mock "github.com/stretchr/testify/mock"
)

// MockDurablePromise is a mock type for the DurablePromise type
type MockDurablePromise struct {
	mock.Mock
	restatecontext.Selectable
}

type MockDurablePromise_Expecter struct {
	mock *mock.Mock
}

func (_m *MockDurablePromise) EXPECT() *MockDurablePromise_Expecter {
	return &MockDurablePromise_Expecter{mock: &_m.Mock}
}

// Peek provides a mock function with given fields: output
func (_m *MockDurablePromise) Peek(output any) (bool, error) {
	ret := _m.Called(output)

	if len(ret) == 0 {
		panic("no return value specified for Peek")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(any) (bool, error)); ok {
		return rf(output)
	}
	if rf, ok := ret.Get(0).(func(any) bool); ok {
		r0 = rf(output)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(any) error); ok {
		r1 = rf(output)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDurablePromise_Peek_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Peek'
type MockDurablePromise_Peek_Call struct {
	*mock.Call
}

// Peek is a helper method to define mock.On call
//   - output any
func (_e *MockDurablePromise_Expecter) Peek(output interface{}) *MockDurablePromise_Peek_Call {
	return &MockDurablePromise_Peek_Call{Call: _e.mock.On("Peek", output)}
}

func (_c *MockDurablePromise_Peek_Call) Run(run func(output any)) *MockDurablePromise_Peek_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockDurablePromise_Peek_Call) Return(ok bool, err error) *MockDurablePromise_Peek_Call {
	_c.Call.Return(ok, err)
	return _c
}

func (_c *MockDurablePromise_Peek_Call) RunAndReturn(run func(any) (bool, error)) *MockDurablePromise_Peek_Call {
	_c.Call.Return(run)
	return _c
}

// Reject provides a mock function with given fields: reason
func (_m *MockDurablePromise) Reject(reason error) error {
	ret := _m.Called(reason)

	if len(ret) == 0 {
		panic("no return value specified for Reject")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(error) error); ok {
		r0 = rf(reason)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDurablePromise_Reject_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Reject'
type MockDurablePromise_Reject_Call struct {
	*mock.Call
}

// Reject is a helper method to define mock.On call
//   - reason error
func (_e *MockDurablePromise_Expecter) Reject(reason interface{}) *MockDurablePromise_Reject_Call {
	return &MockDurablePromise_Reject_Call{Call: _e.mock.On("Reject", reason)}
}

func (_c *MockDurablePromise_Reject_Call) Run(run func(reason error)) *MockDurablePromise_Reject_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(error))
	})
	return _c
}

func (_c *MockDurablePromise_Reject_Call) Return(_a0 error) *MockDurablePromise_Reject_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDurablePromise_Reject_Call) RunAndReturn(run func(error) error) *MockDurablePromise_Reject_Call {
	_c.Call.Return(run)
	return _c
}

// Resolve provides a mock function with given fields: value
func (_m *MockDurablePromise) Resolve(value any) error {
	ret := _m.Called(value)

	if len(ret) == 0 {
		panic("no return value specified for Resolve")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(any) error); ok {
		r0 = rf(value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDurablePromise_Resolve_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Resolve'
type MockDurablePromise_Resolve_Call struct {
	*mock.Call
}

// Resolve is a helper method to define mock.On call
//   - value any
func (_e *MockDurablePromise_Expecter) Resolve(value interface{}) *MockDurablePromise_Resolve_Call {
	return &MockDurablePromise_Resolve_Call{Call: _e.mock.On("Resolve", value)}
}

func (_c *MockDurablePromise_Resolve_Call) Run(run func(value any)) *MockDurablePromise_Resolve_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockDurablePromise_Resolve_Call) Return(_a0 error) *MockDurablePromise_Resolve_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDurablePromise_Resolve_Call) RunAndReturn(run func(any) error) *MockDurablePromise_Resolve_Call {
	_c.Call.Return(run)
	return _c
}

// Result provides a mock function with given fields: output
func (_m *MockDurablePromise) Result(output any) error {
	ret := _m.Called(output)

	if len(ret) == 0 {
		panic("no return value specified for Result")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(any) error); ok {
		r0 = rf(output)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDurablePromise_Result_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Result'
type MockDurablePromise_Result_Call struct {
	*mock.Call
}

// Result is a helper method to define mock.On call
//   - output any
func (_e *MockDurablePromise_Expecter) Result(output interface{}) *MockDurablePromise_Result_Call {
	return &MockDurablePromise_Result_Call{Call: _e.mock.On("Result", output)}
}

func (_c *MockDurablePromise_Result_Call) Run(run func(output any)) *MockDurablePromise_Result_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockDurablePromise_Result_Call) Return(err error) *MockDurablePromise_Result_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockDurablePromise_Result_Call) RunAndReturn(run func(any) error) *MockDurablePromise_Result_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockDurablePromise creates a new instance of MockDurablePromise. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDurablePromise(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDurablePromise {
	mock := &MockDurablePromise{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
