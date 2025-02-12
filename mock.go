package restate

import (
	"github.com/restatedev/sdk-go/internal/state"
)

type mockContext struct {
	state.Context
}

func (m mockContext) inner() state.Context {
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
func WithMockContext(ctx state.Context) mockContext {
	return mockContext{ctx}
}
