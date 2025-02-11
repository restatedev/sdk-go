package mocks

import (
	"github.com/restatedev/sdk-go/internal/futures"
	mock "github.com/stretchr/testify/mock"
)

// MockAwakeableFuture is a mock type for the AwakeableFuture type
type MockAwakeableFuture struct {
	mock.Mock
	futures.Selectable
}

type MockAwakeableFuture_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAwakeableFuture) EXPECT() *MockAwakeableFuture_Expecter {
	return &MockAwakeableFuture_Expecter{mock: &_m.Mock}
}

// Id provides a mock function with no fields
func (_m *MockAwakeableFuture) Id() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Id")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockAwakeableFuture_Id_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Id'
type MockAwakeableFuture_Id_Call struct {
	*mock.Call
}

// Id is a helper method to define mock.On call
func (_e *MockAwakeableFuture_Expecter) Id() *MockAwakeableFuture_Id_Call {
	return &MockAwakeableFuture_Id_Call{Call: _e.mock.On("Id")}
}

func (_c *MockAwakeableFuture_Id_Call) Run(run func()) *MockAwakeableFuture_Id_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockAwakeableFuture_Id_Call) Return(_a0 string) *MockAwakeableFuture_Id_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAwakeableFuture_Id_Call) RunAndReturn(run func() string) *MockAwakeableFuture_Id_Call {
	_c.Call.Return(run)
	return _c
}

// Result provides a mock function with given fields: output
func (_m *MockAwakeableFuture) Result(output any) error {
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

// MockAwakeableFuture_Result_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Result'
type MockAwakeableFuture_Result_Call struct {
	*mock.Call
}

// Result is a helper method to define mock.On call
//   - output any
func (_e *MockAwakeableFuture_Expecter) Result(output interface{}) *MockAwakeableFuture_Result_Call {
	return &MockAwakeableFuture_Result_Call{Call: _e.mock.On("Result", output)}
}

func (_c *MockAwakeableFuture_Result_Call) Run(run func(output any)) *MockAwakeableFuture_Result_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(any))
	})
	return _c
}

func (_c *MockAwakeableFuture_Result_Call) Return(_a0 error) *MockAwakeableFuture_Result_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockAwakeableFuture_Result_Call) RunAndReturn(run func(any) error) *MockAwakeableFuture_Result_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAwakeableFuture creates a new instance of MockAwakeableFuture. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAwakeableFuture(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAwakeableFuture {
	mock := &MockAwakeableFuture{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
