package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	restate "github.com/restatedev/sdk-go"
)

type CommandType int

const (
	SetState CommandType = iota + 1
	GetState
	ClearState
	IncrementStateCounter
	IncrementStateCounterIndirectly
	Sleep
	CallService
	CallSlowService
	IncrementViaDelayedCall
	SideEffect
	ThrowingSideEffect
	SlowSideEffect
	RecoverTerminalCall
	RecoverTerminalMaybeUnAwaited
	AwaitPromise
	ResolveAwakeable
	RejectAwakeable
	IncrementStateCounterViaAwakeable
	CallNextLayerObject
)

type Program struct {
	Commands []InterpreterCommand `json:"commands"`
}

type InterpreterCommand struct {
	Kind     CommandType `json:"kind"`
	Key      *int        `json:"key,omitempty"`
	Duration *int        `json:"duration,omitempty"`
	Index    *int        `json:"index,omitempty"`
	Sleep    *int        `json:"sleep,omitempty"`
	Program  *Program    `json:"program,omitempty"`
}

type InterpreterId struct {
	Key   string `json:"key"`
	Layer uint32 `json:"layer"`
}

func interpret(ctx restate.ObjectContext, layer uint32, value Program) error {
	interpreterId := InterpreterId{
		Layer: layer,
		Key:   restate.Key(ctx),
	}

	for i, cmd := range value.Commands {
		switch cmd.Kind {
		case SetState:
			restate.Set(ctx, fmt.Sprintf("key-%d", *cmd.Key), fmt.Sprintf("value-%d", *cmd.Key))
			break
		case GetState:
			_, err := restate.Get[string](ctx, fmt.Sprintf("key-%d", *cmd.Key))
			if err != nil {
				return err
			}
			break
		case ClearState:
			restate.Clear(ctx, fmt.Sprintf("key-%d", *cmd.Key))
			break
		case IncrementStateCounter:
			count, err := restate.Get[uint32](ctx, "counter")
			if err != nil {
				return err
			}
			count++
			restate.Set(ctx, "counter", count)
			break
		case Sleep:
			if err := restate.Sleep(ctx, time.Duration(*cmd.Duration)*time.Millisecond); err != nil {
				return err
			}
			break
		case CallService:
			expected := fmt.Sprintf("hello-%d", i)
			response, err := restate.Service[string](ctx, "ServiceInterpreterHelper", "echo").
				Request(expected)
			if err != nil {
				return err
			}
			if response != expected {
				return restate.TerminalError(fmt.Errorf("Expected %s but got %s", expected, response))
			}
			break
		case IncrementViaDelayedCall:
			delay := time.Duration(*cmd.Duration) * time.Millisecond
			restate.Service[restate.Void](ctx, "ServiceInterpreterHelper", "incrementIndirectly").Send(interpreterId, restate.WithDelay(delay))
			break
		case IncrementStateCounterIndirectly:
			restate.Service[restate.Void](ctx, "ServiceInterpreterHelper", "incrementIndirectly").Send(interpreterId)
			break
		case CallSlowService:
			expected := fmt.Sprintf("hello-%d", i)
			input := EchoLaterInput{
				Sleep:     *cmd.Sleep,
				Parameter: expected,
			}
			response, err := restate.Service[string](ctx, "ServiceInterpreterHelper", "echoLater").
				Request(input)
			if err != nil {
				return err
			}
			if response != expected {
				return restate.TerminalError(fmt.Errorf("Expected %s but got %s", expected, response))
			}
			break
		case SideEffect:
			expected := fmt.Sprintf("hello-%d", i)
			response, err := restate.Run[string](ctx, func(ctx restate.RunContext) (string, error) { return expected, nil })
			if err != nil {
				return err
			}
			if response != expected {
				return restate.TerminalError(fmt.Errorf("Expected %s but got %s", expected, response))
			}
			break
		case SlowSideEffect:
			err := restate.RunVoid(ctx, func(ctx restate.RunContext) error {
				time.Sleep(1 * time.Millisecond)
				return nil
			})
			if err != nil {
				return err
			}
			break
		case RecoverTerminalCall:
		case RecoverTerminalMaybeUnAwaited:
			_, err := restate.Service[string](ctx, "ServiceInterpreterHelper", "terminalFailure").
				Request(restate.Void{})
			if err == nil {
				return restate.TerminalError(fmt.Errorf("Test assertion failed, was expected to get a terminal error."))
			}
			break
		case ThrowingSideEffect:
			err := restate.RunVoid(ctx, func(ctx restate.RunContext) error {
				if rand.IntN(2) == 1 {
					return fmt.Errorf("Too many 'if err != nil', and no there's no feelings attached to it at all.")
				}
				return nil
			})
			if err != nil {
				return err
			}
			break
		case AwaitPromise:
			// Nothing to do here, we await all the futures eagerly
			break
		case ResolveAwakeable:
			expected := "ok"
			awakeable := restate.Awakeable[string](ctx)
			restate.Service[any](ctx, "ServiceInterpreterHelper", "resolveAwakeable").Send(awakeable.Id())
			response, err := awakeable.Result()
			if err != nil {
				return err
			}
			if response != expected {
				return restate.TerminalError(fmt.Errorf("Expected %s but got %s", expected, response))
			}
			break
		case RejectAwakeable:
			awakeable := restate.Awakeable[string](ctx)
			restate.Service[any](ctx, "ServiceInterpreterHelper", "rejectAwakeable").Send(awakeable.Id())
			_, err := awakeable.Result()
			if err == nil {
				return restate.TerminalError(fmt.Errorf("Test assertion failed, was expected to get a terminal error."))
			}
			break
		case IncrementStateCounterViaAwakeable:
			awakeable := restate.Awakeable[string](ctx)

			restate.Service[restate.Void](ctx, "ServiceInterpreterHelper", "incrementViaAwakeableDance").Send(IncrementViaAwakeableDanceInput{
				Interpreter: interpreterId,
				TxPromiseId: awakeable.Id(),
			})

			theirPromiseIdForUsToResolve, err := awakeable.Result()
			if err != nil {
				return err
			}

			restate.ResolveAwakeable(ctx, theirPromiseIdForUsToResolve, "ok")
			break
		case CallNextLayerObject:
			nextLayer := layer + 1
			objectName := fmt.Sprintf("ObjectInterpreterL%d", nextLayer)
			objectKey := fmt.Sprintf("%d", *cmd.Key)
			program := *cmd.Program

			_, err := restate.Object[restate.Void](ctx, objectName, objectKey, "interpret").
				Request(program)
			if err != nil {
				return err
			}
			break
		default:
			log.Panic("unsupported cmd")
		}
	}

	return nil
}

type EchoLaterInput struct {
	Sleep     int    `json:"sleep"`
	Parameter string `json:"parameter"`
}

type IncrementViaAwakeableDanceInput struct {
	Interpreter InterpreterId `json:"interpreter"`
	TxPromiseId string        `json:"txPromiseId"`
}

func init() {
	// iterate 3 times
	for i := 0; i < 3; i++ {
		REGISTRY.AddDefinition(
			restate.NewObject(fmt.Sprintf("ObjectInterpreterL%d", i)).
				Handler("interpret", restate.NewObjectHandler(
					func(ctx restate.ObjectContext, value Program) (restate.Void, error) {
						err := interpret(ctx, uint32(i), value)
						return restate.Void{}, err
					})))
	}
	REGISTRY.AddDefinition(
		restate.NewService("ServiceInterpreterHelper").
			Handler("ping", restate.NewServiceHandler(
				func(ctx restate.Context, parameters string) (restate.Void, error) {
					return restate.Void{}, nil
				})).
			Handler("echo", restate.NewServiceHandler(
				func(ctx restate.Context, parameters string) (string, error) {
					return parameters, nil
				})).
			Handler("echoLater", restate.NewServiceHandler(
				func(ctx restate.Context, parameters EchoLaterInput) (string, error) {
					restate.Sleep(ctx, time.Duration(parameters.Sleep)*time.Millisecond)
					return parameters.Parameter, nil
				})).
			Handler("terminalFailure", restate.NewServiceHandler(
				func(ctx restate.Context, _ restate.Void) (string, error) {
					return "", restate.TerminalErrorf("bye")
				})).
			Handler("incrementIndirectly", restate.NewServiceHandler(
				func(ctx restate.Context, id InterpreterId) (restate.Void, error) {
					objectName := fmt.Sprintf("ObjectInterpreterL%d", id.Layer)
					program := Program{Commands: []InterpreterCommand{{
						Kind: IncrementStateCounter,
					}}}

					restate.Object[restate.Void](ctx, objectName, id.Key, "interpret").Send(program)

					return restate.Void{}, nil
				})).
			Handler("resolveAwakeable", restate.NewServiceHandler(
				func(ctx restate.Context, id string) (restate.Void, error) {
					restate.ResolveAwakeable(ctx, id, "ok")
					return restate.Void{}, nil
				})).
			Handler("rejectAwakeable", restate.NewServiceHandler(
				func(ctx restate.Context, id string) (restate.Void, error) {
					restate.RejectAwakeable(ctx, id, restate.TerminalErrorf("error"))
					return restate.Void{}, nil
				})).
			Handler("incrementViaAwakeableDance", restate.NewServiceHandler(
				func(ctx restate.Context, input IncrementViaAwakeableDanceInput) (restate.Void, error) {
					awakeable := restate.Awakeable[string](ctx)

					restate.ResolveAwakeable(ctx, input.TxPromiseId, awakeable.Id())

					_, err := awakeable.Result()
					if err != nil {
						return restate.Void{}, err
					}

					objectName := fmt.Sprintf("ObjectInterpreterL%d", input.Interpreter.Layer)
					program := Program{Commands: []InterpreterCommand{{
						Kind: IncrementStateCounter,
					}}}

					restate.Object[restate.Void](ctx, objectName, input.Interpreter.Key, "interpret").Send(program)

					return restate.Void{}, nil
				})))
}
