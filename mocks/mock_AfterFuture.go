package mocks

import (
	mock "github.com/stretchr/testify/mock"

	"github.com/restatedev/sdk-go/internal/futures"
)

// MockAfterFuture is a mock type for the AfterFuture type
type MockAfterFuture struct {
	mock.Mock
	futures.Selectable
}

type MockAfterFuture_Expecter struct {
	parent *MockAfterFuture
	mock   *mock.Mock
}

func (_m *MockAfterFuture) EXPECT() *MockAfterFuture_Expecter {
	return &MockAfterFuture_Expecter{parent: _m, mock: &_m.Mock}
}

// Done provides a mock function with no fields
func (_m *MockAfterFuture) Done() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Done")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockAfterFuture_Done_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Done'
type MockAfterFuture_Done_Call struct {
	*mock.Call
}

// Done is a helper method to define mock.On call
func (_e *MockAfterFuture_Expecter) Done() *MockAfterFuture_Done_Call {
	return &MockAfterFuture_Done_Call{Call: _e.mock.On("Done")}
}

func (_c *MockAfterFuture_Done_Call) Run(run func()) *MockAfterFuture_Done_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockAfterFuture_Done_Call) Return(_a0 error) *MockAfterFuture_Done_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAfterFuture_Done_Call) RunAndReturn(run func() error) *MockAfterFuture_Done_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAfterFuture creates a new instance of MockAfterFuture. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAfterFuture(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAfterFuture {
	mock := &MockAfterFuture{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
