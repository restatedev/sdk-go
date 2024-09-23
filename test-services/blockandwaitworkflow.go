package main

import (
	restate "github.com/restatedev/sdk-go"
)

const MY_STATE = "my-state"
const MY_DURABLE_PROMISE = "durable-promise"

func init() {
	REGISTRY.AddDefinition(
		restate.NewWorkflow("BlockAndWaitWorkflow").
			Handler("run", restate.NewWorkflowHandler(
				func(ctx restate.WorkflowContext, input string) (string, error) {
					restate.Set(ctx, MY_STATE, input)
					output, err := restate.Promise[string](ctx, MY_DURABLE_PROMISE).Result()
					if err != nil {
						return "", err
					}

					peek, err := restate.Promise[*string](ctx, MY_DURABLE_PROMISE).Peek()
					if peek == nil {
						return "", restate.TerminalErrorf("Durable promise should be completed")
					}

					return output, nil
				})).
			Handler("unblock", restate.NewWorkflowSharedHandler(
				func(ctx restate.WorkflowSharedContext, output string) (restate.Void, error) {
					return restate.Void{}, restate.Promise[string](ctx, MY_DURABLE_PROMISE).Resolve(output)
				})).
			Handler("getState", restate.NewWorkflowSharedHandler(
				func(ctx restate.WorkflowSharedContext, input restate.Void) (*string, error) {
					return restate.Get[*string](ctx, MY_STATE)
				})))
}
