package client

import (
	"context"
	"net/url"

	"github.com/restatedev/sdk-go/internal/options"
)

// Workflow gets a Workflow request client by service name, workflow ID and method name
// It must be called with a context returned from [Connect]
func Workflow[O any](ctx context.Context, service string, workflowID string, method string, opts ...options.IngressClientOption) IngressClient[any, O] {
	return Object[O](ctx, service, workflowID, method, opts...)
}

type WorkflowSubmission struct {
	InvocationId string
}

func (w WorkflowSubmission) attachable() bool {
	return true
}

func (w WorkflowSubmission) attachUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "invocation", w.InvocationId, "attach")
}

func (w WorkflowSubmission) outputUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "invocation", w.InvocationId, "output")
}

var _ Attacher = WorkflowSubmission{}

// WorkflowSubmit submits a workflow, defaulting to 'Run' as the main handler name, but this is configurable with [restate.WithWorkflowRun]
// It must be called with a context returned from [Connect]
func WorkflowSubmit[I any](ctx context.Context, service string, workflowID string, input I, opts ...options.WorkflowSubmitOption) (WorkflowSubmission, error) {
	os := options.WorkflowSubmitOptions{}
	for _, opt := range opts {
		opt.BeforeWorkflowSubmit(&os)
	}
	if os.RunHandler == "" {
		os.RunHandler = "Run"
	}

	send, err := Workflow[I](ctx, service, workflowID, os.RunHandler, os).Send(input, os)
	if err != nil {
		return WorkflowSubmission{}, err
	}
	return WorkflowSubmission{InvocationId: send.InvocationId}, nil
}

type WorkflowIdentifier struct {
	Service    string
	WorkflowID string
}

var _ Attacher = WorkflowIdentifier{}

func (w WorkflowIdentifier) attachable() bool {
	return true
}

func (w WorkflowIdentifier) attachUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "workflow", w.Service, w.WorkflowID, "attach")
}

func (w WorkflowIdentifier) outputUrl(connURL *url.URL) *url.URL {
	return connURL.JoinPath("restate", "workflow", w.Service, w.WorkflowID, "output")
}

// WorkflowAttach attaches to a workflow, waiting for it to complete and returning the result.
// It is only possible to 'attach' to a workflow that has been previously submitted.
// This operation is safe to retry many times, and it will always return the same result.
// It must be called with a context returned from [Connect]
func WorkflowAttach[O any](ctx context.Context, service string, workflowID string, opts ...options.IngressClientOption) (O, error) {
	return Attach[O](ctx, WorkflowIdentifier{service, workflowID}, opts...)
}

// WorkflowOutput tries to retrieve the output of a workflow if it has already completed. Otherwise, [ready] will be false.
// It must be called with a context returned from [Connect]
func WorkflowOutput[O any](ctx context.Context, service string, workflowID string, opts ...options.IngressClientOption) (o O, ready bool, err error) {
	return GetOutput[O](ctx, WorkflowIdentifier{service, workflowID}, opts...)
}
