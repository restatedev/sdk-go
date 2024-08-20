package restate_test

import (
	"context"
	"testing"

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
}

type expectedMethods = map[string]*internal.ServiceHandlerType

var exclusive = internal.ServiceHandlerType_EXCLUSIVE
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
	{rcvr: mixed{}, shouldPanic: true},
	{rcvr: empty{}, shouldPanic: true},
}

func TestReflect(t *testing.T) {
	for _, test := range tests {
		t.Run(test.serviceName, func(t *testing.T) {
			defer func() {
				if err := recover(); err != nil {
					if test.shouldPanic {
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
				foundMethods[k] = foundHandler.HandlerType()
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

func (validObject) SkipInvalidCtx(ctx context.Context, _ string) (string, error) {
	return "", nil
}

func (validObject) SkipInvalidError(ctx restate.ObjectContext, _ string) (error, string) {
	return nil, ""
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
