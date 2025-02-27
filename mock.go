package restate

import (
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

type mockContext struct {
	restatecontext.Context
}

func (m mockContext) inner() restatecontext.Context {
	return m.Context
}
func (m mockContext) object()          {}
func (m mockContext) exclusiveObject() {}
func (m mockContext) workflow()        {}
func (m mockContext) runWorkflow()     {}

var _ RunContext = mockContext{}
var _ Context = mockContext{}
var _ ObjectSharedContext = mockContext{}
var _ ObjectContext = mockContext{}
var _ WorkflowSharedContext = mockContext{}
var _ WorkflowContext = mockContext{}

// WithMockContext allows providing a mocked state.Context to handlers
func WithMockContext(ctx restatecontext.Context) mockContext {
	return mockContext{ctx}
}
