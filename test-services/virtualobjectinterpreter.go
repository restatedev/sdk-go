package main

import (
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
)

const RESULTS = "results"

type InterpretRequest struct {
	Commands []Command `json:"commands"`
}

type Command struct {
	Type          string             `json:"type"`
	Commands      []AwaitableCommand `json:"commands,omitempty"`
	Command       AwaitableCommand   `json:"command,omitempty"`
	AwakeableKey  string             `json:"awakeableKey,omitempty"`
	TimeoutMillis uint64             `json:"timeoutMillis,omitempty"`
	EnvName       string             `json:"envName,omitempty"`
	Value         string             `json:"value,omitempty"`
	Reason        string             `json:"reason,omitempty"`
}

type AwaitableCommand struct {
	Type          string `json:"type"`
	AwakeableKey  string `json:"awakeableKey,omitempty"`
	TimeoutMillis uint64 `json:"timeoutMillis,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type ResolveAwakeableRequest struct {
	AwakeableKey string `json:"awakeableKey,omitempty"`
	Value        string `json:"value,omitempty"`
}

type RejectAwakeableRequest struct {
	AwakeableKey string `json:"awakeableKey,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

func init() {
	REGISTRY.AddDefinition(
		restate.NewObject("VirtualObjectCommandInterpreter").
			Handler("hasAwakeable", restate.NewObjectSharedHandler(
				func(ctx restate.ObjectSharedContext, key string) (bool, error) {
					id, err := restate.Get[string](ctx, awakeableStateKey(key))
					if err != nil {
						return false, err
					}
					return id != "", nil
				})).
			Handler("resolveAwakeable", restate.NewObjectSharedHandler(resolveAwakeableHandler)).
			Handler("rejectAwakeable", restate.NewObjectSharedHandler(rejectAwakeableHandler)).
			Handler("getResults", restate.NewObjectSharedHandler(
				func(ctx restate.ObjectSharedContext, _ restate.Void) ([]string, error) {
					return restate.Get[[]string](ctx, RESULTS)
				})).
			Handler("interpretCommands", restate.NewObjectHandler(
				func(ctx restate.ObjectContext, req InterpretRequest) (string, error) {
					var err error
					lastResult := ""

					for _, command := range req.Commands {
						switch command.Type {
						case "awaitAny":
							lastResult, err = awaitAnyCommand(ctx, command.Commands)
							if err != nil {
								return "", err
							}
							break
						case "awaitAnySuccessful":
							lastResult, err = awaitAnySuccessfulCommand(ctx, command.Commands)
							if err != nil {
								return "", err
							}
							break
						case "awaitOne":
							lastResult, err = awaitableCommandResult(command.Command.toFuture(ctx))
							if err != nil {
								return "", err
							}
							break
						case "resolveAwakeable":
							_, err = resolveAwakeableHandler(ctx, ResolveAwakeableRequest{
								AwakeableKey: command.AwakeableKey,
								Value:        command.Value,
							})
							if err != nil {
								return "", err
							}
							lastResult = ""
							break
						case "rejectAwakeable":
							_, err = rejectAwakeableHandler(ctx, RejectAwakeableRequest{
								AwakeableKey: command.AwakeableKey,
								Reason:       command.Reason,
							})
							if err != nil {
								return "", err
							}
							lastResult = ""
							break
						case "awaitAwakeableOrTimeout":
							awk := restate.Awakeable[string](ctx)
							restate.Set(ctx, awakeableStateKey(command.AwakeableKey), awk.Id())

							timeout := restate.After(ctx, time.Duration(command.TimeoutMillis)*time.Millisecond)
							selector := restate.Select(ctx, timeout, awk)
							switch selector.Select() {
							case timeout:
								return "", restate.TerminalErrorf("await-timeout")
							case awk:
								lastResult, err = awk.Result()
								if err != nil {
									return "", err
								}
								break
							default:
								panic("there are no other futures selected here?!")
							}
						case "getEnvVariable":
							lastResult, err = getEnvVariable(ctx, command.EnvName)
							if err != nil {
								return "", err
							}
							break
						}

						resultsList, err := restate.Get[[]string](ctx, RESULTS)
						if err != nil {
							return "", err
						}
						resultsList = append(resultsList, lastResult)
						restate.Set(ctx, RESULTS, resultsList)
					}

					return lastResult, nil
				})))
}

func rejectAwakeableHandler(ctx restate.ObjectSharedContext, req RejectAwakeableRequest) (restate.Void, error) {
	id, err := restate.Get[string](ctx, awakeableStateKey(req.AwakeableKey))
	if err != nil {
		return restate.Void{}, err
	}
	if id == "" {
		return restate.Void{}, restate.TerminalErrorf("awakeable is not registered yet")
	}
	restate.RejectAwakeable(ctx, id, restate.TerminalErrorf("%s", req.Reason))
	return restate.Void{}, nil
}

func resolveAwakeableHandler(ctx restate.ObjectSharedContext, req ResolveAwakeableRequest) (restate.Void, error) {
	id, err := restate.Get[string](ctx, awakeableStateKey(req.AwakeableKey))
	if err != nil {
		return restate.Void{}, err
	}
	if id == "" {
		return restate.Void{}, restate.TerminalErrorf("awakeable is not registered yet")
	}
	restate.ResolveAwakeable(ctx, id, req.Value)
	return restate.Void{}, nil
}

func awaitAnyCommand(ctx restate.ObjectContext, commands []AwaitableCommand) (string, error) {
	var selectables []restate.Selectable
	for _, cmd := range commands {
		selectables = append(selectables, cmd.toFuture(ctx))
	}
	return awaitableCommandResult(restate.Select(ctx, selectables...).Select())
}

func awaitAnySuccessfulCommand(ctx restate.ObjectContext, commands []AwaitableCommand) (string, error) {
	var selectables []restate.Selectable
	for _, cmd := range commands {
		selectables = append(selectables, cmd.toFuture(ctx))
	}
	selector := restate.Select(ctx, selectables...)
	for selector.Remaining() {
		selected := selector.Select()
		switch selected.(type) {
		case restate.AwakeableFuture[string]:
			res, err := selected.(restate.AwakeableFuture[string]).Result()
			if err != nil {
				continue
			}
			return res, err
		case restate.AfterFuture:
			err := selected.(restate.AfterFuture).Done()
			if err != nil {
				continue
			}
			return "sleep", err
		case restate.RunAsyncFuture[string]:
			res, err := selected.(restate.RunAsyncFuture[string]).Result()
			if err != nil {
				continue
			}
			return res, err
		default:
			panic("Unsupported future type")
		}
	}
	return "", restate.TerminalErrorf("No future selected")
}

func awaitableCommandResult(selected restate.Selectable) (string, error) {
	switch selected.(type) {
	case restate.AwakeableFuture[string]:
		res, err := selected.(restate.AwakeableFuture[string]).Result()
		return res, err
	case restate.AfterFuture:
		err := selected.(restate.AfterFuture).Done()
		return "sleep", err
	case restate.RunAsyncFuture[string]:
		res, err := selected.(restate.RunAsyncFuture[string]).Result()
		return res, err
	default:
		panic("Unsupported future type")
	}
}

func awakeableStateKey(awkKey string) string {
	return fmt.Sprintf("awk-%s", awkKey)
}

func (cmd AwaitableCommand) toFuture(ctx restate.ObjectContext) restate.Selectable {
	switch cmd.Type {
	case "createAwakeable":
		awk := restate.Awakeable[string](ctx)
		restate.Set(ctx, awakeableStateKey(cmd.AwakeableKey), awk.Id())
		return awk
	case "runThrowTerminalException":
		return restate.RunAsync[string](ctx, func(ctx restate.RunContext) (string, error) {
			return "", restate.TerminalErrorf("%s", cmd.Reason)
		})
	case "sleep":
		return restate.After(ctx, time.Duration(cmd.TimeoutMillis)*time.Millisecond)
	default:
		panic(fmt.Sprintf("Unsupported command %s", cmd.Type))
	}
}
