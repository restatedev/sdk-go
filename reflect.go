package restate

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/restatecontext"
)

type serviceNamer interface {
	ServiceName() string
}

var (
	typeOfContext               = reflect.TypeOf((*Context)(nil)).Elem()
	typeOfObjectContext         = reflect.TypeOf((*ObjectContext)(nil)).Elem()
	typeOfSharedObjectContext   = reflect.TypeOf((*ObjectSharedContext)(nil)).Elem()
	typeOfWorkflowContext       = reflect.TypeOf((*WorkflowContext)(nil)).Elem()
	typeOfSharedWorkflowContext = reflect.TypeOf((*WorkflowSharedContext)(nil)).Elem()
	typeOfError                 = reflect.TypeOf((*error)(nil)).Elem()
)

func acceptedContextParameterString(serviceType internal.ServiceType) string {
	switch serviceType {
	case internal.ServiceType_SERVICE:
		return typeOfContext.String()
	case internal.ServiceType_VIRTUAL_OBJECT:
		return fmt.Sprintf("%s or %s", typeOfObjectContext, typeOfSharedObjectContext)
	case internal.ServiceType_WORKFLOW:
		return fmt.Sprintf("%s or %s", typeOfWorkflowContext, typeOfSharedWorkflowContext)
	}
	return ""
}

// Reflect converts a struct with methods into a service definition where each correctly-typed
// and exported method of the struct will become a handler in the definition. The service name
// defaults to the name of the struct, but this can be overidden by providing a `ServiceName() string` method.
// The handler name is the name of the method. Handler methods should have one of the following signatures:
// - (ctx, I) (O, error)
// - (ctx, I) (O)
// - (ctx, I) (error)
// - (ctx, I)
// - (ctx)
// - (ctx) (error)
// - (ctx) (O)
// - (ctx) (O, error)
// Where ctx is [WorkflowContext], [WorkflowSharedContext], [ObjectContext], [ObjectSharedContext] or [Context]. Other signatures are ignored.
// Signatures without an I or O type will be treated as if [Void] was provided.
// This function will panic if a mixture of object service and workflow method signatures or opts are provided, or if multiple WorkflowContext
// methods are defined.
//
// Input types will be deserialised with the provided codec (defaults to JSON) except when they are [Void],
// in which case no input bytes or content type may be sent.
// Output types will be serialised with the provided codec (defaults to JSON) except when they are [Void],
// in which case no data will be sent and no content type set.
func Reflect(rcvr any, opts ...options.ServiceDefinitionOption) ServiceDefinition {
	typ := reflect.TypeOf(rcvr)
	val := reflect.ValueOf(rcvr)
	var name string
	if sn, ok := rcvr.(serviceNamer); ok {
		name = sn.ServiceName()
	} else {
		name = reflect.Indirect(val).Type().Name()
	}

	var definition ServiceDefinition
	var foundWorkflowRun bool

	type skippedHandler struct {
		service string
		handler string
		reason  string
	}

	skipped := []skippedHandler{}

	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if !method.IsExported() {
			skipped = append(skipped, skippedHandler{service: name, handler: mname, reason: "Not exported"})
			continue
		}

		if mtype.NumIn() < 2 {
			skipped = append(skipped, skippedHandler{service: name, handler: mname, reason: "Incorrect number of parameters; should be 1 or 2"})
			continue
		}

		var handlerType internal.ServiceHandlerType
		switch mtype.In(1) {
		case typeOfContext:
			if definition == nil {
				definition = NewService(name, opts...)
			} else if definition.Type() != internal.ServiceType_SERVICE {
				panic(fmt.Sprintf("error when adding handler '%s/%s': the function declares %s as first parameter, but service type is '%s', only %s are accepted", name, mname, typeOfContext.String(), definition.Type(), acceptedContextParameterString(definition.Type())))
			}
		case typeOfObjectContext:
			if definition == nil {
				definition = NewObject(name, opts...)
			} else if definition.Type() != internal.ServiceType_VIRTUAL_OBJECT {
				panic(fmt.Sprintf("error when adding handler '%s/%s': the function declares %s as first parameter, but service type is '%s', only %s are accepted", name, mname, typeOfObjectContext.String(), definition.Type(), acceptedContextParameterString(definition.Type())))
			}
			handlerType = internal.ServiceHandlerType_EXCLUSIVE
		case typeOfSharedObjectContext:
			if definition == nil {
				definition = NewObject(name, opts...)
			} else if definition.Type() != internal.ServiceType_VIRTUAL_OBJECT {
				panic(fmt.Sprintf("error when adding handler '%s/%s': the function declares %s as first parameter, but service type is '%s', only %s are accepted", name, mname, typeOfSharedObjectContext.String(), definition.Type(), acceptedContextParameterString(definition.Type())))
			}
			handlerType = internal.ServiceHandlerType_SHARED
		case typeOfWorkflowContext:
			if definition == nil {
				definition = NewWorkflow(name, opts...)
			} else if definition.Type() != internal.ServiceType_WORKFLOW {
				panic(fmt.Sprintf("error when adding handler '%s/%s': the function declares %s as first parameter, but service type is '%s', only %s are accepted", name, mname, typeOfWorkflowContext.String(), definition.Type(), acceptedContextParameterString(definition.Type())))
			} else if foundWorkflowRun {
				panic(fmt.Sprintf("error when adding handler '%s/%s': found more than one WorkflowContext argument; a workflow may only have one 'Run' method, the rest must be WorkflowSharedContext.", name, mname))
			}
			handlerType = internal.ServiceHandlerType_WORKFLOW
			foundWorkflowRun = true
		case typeOfSharedWorkflowContext:
			if definition == nil {
				definition = NewWorkflow(name, opts...)
			} else if definition.Type() != internal.ServiceType_WORKFLOW {
				panic(fmt.Sprintf("error when adding handler '%s/%s': the function declares %s as first parameter, but service type is '%s', only %s are accepted", name, mname, typeOfSharedWorkflowContext.String(), definition.Type(), acceptedContextParameterString(definition.Type())))
			}
			handlerType = internal.ServiceHandlerType_SHARED
		default:
			// first parameter is not a context
			skipped = append(skipped, skippedHandler{service: name, handler: mname, reason: "First parameter is not a restate context object"})
			continue
		}

		// if we are here, we have an exported method with a restate context; most likely this was intended as a restate handler, so issues from here on should panic, not continue

		// Method needs 2-3 ins: receiver, Context, optionally I
		var input reflect.Type
		switch mtype.NumIn() {
		case 2:
			// (ctx)
			input = nil
		case 3:
			// (ctx, I)
			input = mtype.In(2)
		default:
			panic(fmt.Sprintf("Incorrect number of arguments for handler '%s/%s': a restate handler should have a ctx parameter and optionally *one* input parameter", name, mname))
		}

		// Method needs 0-2 outs: (), (O), (error), (O, error) are all valid
		var output reflect.Type
		var hasError bool
		switch mtype.NumOut() {
		case 0:
			// ()
			output = nil
			hasError = false
		case 1:
			if returnType := mtype.Out(0); returnType == typeOfError {
				// (error)
				output = nil
				hasError = true
			} else {
				// (O)
				output = returnType
				hasError = false
			}
		case 2:
			if returnType := mtype.Out(1); returnType != typeOfError {
				panic(fmt.Sprintf("Incorrect returns for method handler '%s/%s': if returning two parameters from a restate handler, the second must be an error", name, mname))
			}
			// (O, error)
			output = mtype.Out(0)
			hasError = true
		default:
			panic(fmt.Sprintf("Incorrect returns for handler '%s/%s': at most 2 parameters are allowed in a restate handler (result and error)", name, mname))
		}

		switch def := definition.(type) {
		case *service:
			def.Handler(mname, &reflectHandler{
				fn:          method.Func,
				receiver:    val,
				input:       input,
				output:      output,
				hasError:    hasError,
				options:     options.HandlerOptions{},
				handlerType: nil,
			},
			)
		case *object:
			def.Handler(mname, &reflectHandler{
				fn:          method.Func,
				receiver:    val,
				input:       input,
				output:      output,
				hasError:    hasError,
				options:     options.HandlerOptions{},
				handlerType: &handlerType,
			},
			)
		case *workflow:
			def.Handler(mname, &reflectHandler{
				fn:          method.Func,
				receiver:    val,
				input:       input,
				output:      output,
				hasError:    hasError,
				options:     options.HandlerOptions{},
				handlerType: &handlerType,
			},
			)
		}
	}

	if definition == nil || len(definition.Handlers()) == 0 {
		panic(fmt.Sprintf("no valid handlers could be found within the exported methods on this struct. Please ensure that methods are defined on the exact type T provided to .Reflect, and not on *T. Skipped handlers: %+v", skipped))
	}

	if definition.Type() == internal.ServiceType_WORKFLOW && !foundWorkflowRun {
		panic(fmt.Sprintf("no WorkflowContext method found for workflow '%s'; a workflow must have exactly one 'Run' handler", name))
	}

	return definition
}

type reflectHandler struct {
	fn          reflect.Value
	receiver    reflect.Value
	input       reflect.Type
	output      reflect.Type
	hasError    bool
	options     options.HandlerOptions
	handlerType *internal.ServiceHandlerType
}

func (h *reflectHandler) GetOptions() *options.HandlerOptions {
	return &h.options
}

func (h *reflectHandler) InputPayload() *encoding.InputPayload {
	if h.input == nil {
		return encoding.InputPayloadFor(h.options.Codec, Void{})
	}
	return encoding.InputPayloadFor(h.options.Codec, reflect.Zero(h.input).Interface())
}

func (h *reflectHandler) OutputPayload() *encoding.OutputPayload {
	if h.output == nil {
		return encoding.OutputPayloadFor(h.options.Codec, Void{})
	}
	return encoding.OutputPayloadFor(h.options.Codec, reflect.Zero(h.output).Interface())
}

func (h *reflectHandler) HandlerType() *internal.ServiceHandlerType {
	return h.handlerType
}

func (h *reflectHandler) Call(ctx restatecontext.Context, bytes []byte) ([]byte, error) {

	var args []reflect.Value
	if h.input != nil {
		input := reflect.New(h.input)

		if err := encoding.Unmarshal(h.options.Codec, bytes, input.Interface()); err != nil {
			return nil, TerminalError(fmt.Errorf("request could not be decoded into handler input type: %w", err), http.StatusBadRequest)
		}

		args = []reflect.Value{h.receiver, reflect.ValueOf(ctxWrapper{ctx}), input.Elem()}
	} else {
		args = []reflect.Value{h.receiver, reflect.ValueOf(ctxWrapper{ctx})}
	}

	output := h.fn.Call(args)
	var outI any

	switch [2]bool{h.output != nil, h.hasError} {
	case [2]bool{false, false}:
		// ()
		return nil, nil
	case [2]bool{false, true}:
		// (error)
		errI := output[0].Interface()
		if errI != nil {
			return nil, errI.(error)
		}
		return nil, nil
	case [2]bool{true, false}:
		// (O)
		outI = output[0].Interface()
	case [2]bool{true, true}:
		// (O, error)
		errI := output[1].Interface()
		if errI != nil {
			return nil, errI.(error)
		}
		outI = output[0].Interface()
	}

	bytes, err := encoding.Marshal(h.options.Codec, outI)
	if err != nil {
		// we don't use a terminal error here as this is hot-fixable by changing the return type
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	return bytes, nil
}

var _ restatecontext.Handler = (*reflectHandler)(nil)
