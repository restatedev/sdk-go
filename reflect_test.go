package restate_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal"
	"github.com/stretchr/testify/require"
)

type reflectTestParams struct {
	rcvr            interface{}
	opts            []restate.ServiceDefinitionOption
	serviceName     string
	serviceType     internal.ServiceType
	expectedMethods expectedMethods
	shouldPanic     bool
	panicContains   string
}

type expectedMethods = map[string]*internal.ServiceHandlerType

var exclusive = internal.ServiceHandlerType_EXCLUSIVE
var workflowRun = internal.ServiceHandlerType_WORKFLOW
var shared = internal.ServiceHandlerType_SHARED

var tests []reflectTestParams = []reflectTestParams{
	{rcvr: validObject{}, serviceName: "validObject", expectedMethods: expectedMethods{
		"Greet":                  &exclusive,
		"GreetShared":            &shared,
		"NoInput":                &exclusive,
		"NoError":                &exclusive,
		"NoOutput":               &exclusive,
		"NoOutputNoError":        &exclusive,
		"NoInputNoError":         &exclusive,
		"NoInputNoOutput":        &exclusive,
		"NoInputNoOutputNoError": &exclusive,
	}},
	{rcvr: validService{}, serviceName: "validService", expectedMethods: expectedMethods{
		"Greet": nil,
	}},
	{rcvr: namedService{}, serviceName: "foobar", expectedMethods: expectedMethods{
		"Greet": nil,
	}},
	{rcvr: validWorkflow{}, serviceName: "validWorkflow", expectedMethods: expectedMethods{
		"Run":    &workflowRun,
		"Status": &shared,
	}},
	{rcvr: mixed{}, shouldPanic: true, panicContains: "error when adding handler 'mixed/GreetShared': the function declares restate.ObjectSharedContext as first parameter, but service type is 'SERVICE', only restate.Context are accepted"},
	{rcvr: empty{}, shouldPanic: true, panicContains: "no valid handlers could be found"},
	{rcvr: notExported{}, shouldPanic: true, panicContains: "no valid handlers could be found"},
	{rcvr: firstParamNotContext{}, shouldPanic: true, panicContains: "no valid handlers could be found"},
	{rcvr: secondReturnNotError{}, shouldPanic: true, panicContains: "the second must be an error"},
	{rcvr: tooManyReturns{}, shouldPanic: true, panicContains: "at most 2 parameters are allowed"},
}

func TestReflect(t *testing.T) {
	for _, test := range tests {
		t.Run(test.serviceName, func(t *testing.T) {
			defer func() {
				if err := recover(); err != nil {
					if test.shouldPanic {
						require.Contains(t, fmt.Sprintf("%v", err), test.panicContains)
						return
					} else {
						panic(err)
					}
				} else if test.shouldPanic {
					t.Fatal("test should have panicked")
				}
			}()
			def := restate.Reflect(test.rcvr, test.opts...)
			foundMethods := make(map[string]*internal.ServiceHandlerType, len(def.Handlers()))
			for k, foundHandler := range def.Handlers() {
				t.Run(k, func(t *testing.T) {
					foundMethods[k] = foundHandler.HandlerType()
					// check for panics
					_ = foundHandler.InputPayload()
					_ = foundHandler.OutputPayload()
					_, err := foundHandler.Call(nil, []byte(`""`))
					require.NoError(t, err)
				})
			}
			require.Equal(t, test.expectedMethods, foundMethods)
			require.Equal(t, test.serviceName, def.Name())
		})
	}
}

type validObject struct{}

func (validObject) Greet(ctx restate.ObjectContext, _ string) (string, error) {
	return "", nil
}

func (validObject) GreetShared(ctx restate.ObjectSharedContext, _ string) (string, error) {
	return "", nil
}

func (validObject) NoInput(ctx restate.ObjectContext) (string, error) {
	return "", nil
}

func (validObject) NoError(ctx restate.ObjectContext, _ string) string {
	return ""
}

func (validObject) NoOutput(ctx restate.ObjectContext, _ string) error {
	return nil
}

func (validObject) NoOutputNoError(ctx restate.ObjectContext, _ string) {
}

func (validObject) NoInputNoError(ctx restate.ObjectContext) string {
	return ""
}

func (validObject) NoInputNoOutput(ctx restate.ObjectContext) error {
	return nil
}

func (validObject) NoInputNoOutputNoError(ctx restate.ObjectContext) {
}

func (validObject) SkipNoArguments() (string, error) {
	return "", nil
}

func (validObject) SkipInvalidCtx(ctx context.Context, _ string) (string, error) {
	return "", nil
}

func (validObject) skipUnexported(_ string) (string, error) {
	return "", nil
}

type validService struct{}

func (validService) Greet(ctx restate.Context, _ string) (string, error) {
	return "", nil
}

type namedService struct{}

func (namedService) ServiceName() string {
	return "foobar"
}

type validWorkflow struct{}

func (validWorkflow) Run(ctx restate.WorkflowContext) error {
	return nil
}

func (validWorkflow) Status(ctx restate.WorkflowSharedContext, _ string) (string, error) {
	return "", nil
}

func (namedService) Greet(ctx restate.Context, _ string) (string, error) {
	return "", nil
}

type mixed struct{}

func (mixed) Greet(ctx restate.Context, _ string) (string, error) {
	return "", nil
}
func (mixed) GreetShared(ctx restate.ObjectSharedContext, _ string) (string, error) {
	return "", nil
}

type empty struct{}

type notExported struct{}

func (notExported) notExported(ctx restate.Context) {}

type firstParamNotContext struct{}

func (firstParamNotContext) FirstParamNotContext(foo string) {}

type secondReturnNotError struct{}

func (secondReturnNotError) SecondReturnNotError(ctx restate.Context) (string, string) {
	return "", ""
}

type tooManyReturns struct{}

func (tooManyReturns) TooManyReturns(ctx restate.Context) (string, string, string) {
	return "", "", ""
}

func TestReflectWithHandlerOptions(t *testing.T) {
	service := restate.Reflect(validService{}).
		ConfigureHandler("Greet", restate.WithEnableLazyState(true))

	require.Equal(t, *service.Handlers()["Greet"].GetOptions().EnableLazyState, true)
}

func TestReflectWithHandlerOptionsWorkflow(t *testing.T) {
	service := restate.Reflect(validWorkflow{}).
		ConfigureHandler("Run", restate.WithWorkflowRetention(10*time.Second))

	require.Equal(t, *service.Handlers()["Run"].GetOptions().WorkflowRetention, 10*time.Second)
}
