package restatecontext

import (
	"context"
	pbinternal "github.com/restatedev/sdk-go/internal/generated"
	"github.com/restatedev/sdk-go/internal/log"
	"github.com/restatedev/sdk-go/internal/options"
	"github.com/restatedev/sdk-go/internal/rand"
	"github.com/restatedev/sdk-go/internal/statemachine"
	"github.com/restatedev/sdk-go/rcontext"
	"io"
	"log/slog"
	"sync/atomic"
	"time"
)

type Request struct {
	// The unique id that identifies the current function invocation. This id is guaranteed to be
	// unique across invocations, but constant across reties and suspensions.
	ID string
	// Request headers - the following headers capture the original invocation headers, as provided to
	// the ingress.
	Headers map[string]string
	// Attempt headers - the following headers are sent by the restate runtime.
	// These headers are attempt specific, generated by the restate runtime uniquely for each attempt.
	// These headers might contain information such as the W3C trace context, and attempt specific information.
	AttemptHeaders map[string][]string
	// Raw unparsed request body
	Body []byte
}

type Context interface {
	context.Context
	Log() *slog.Logger
	Request() *Request

	// available outside of .Run()
	Rand() rand.Rand
	Sleep(d time.Duration, opts ...options.SleepOption) error
	After(d time.Duration, opts ...options.SleepOption) AfterFuture
	Service(service, method string, options ...options.ClientOption) Client
	Object(service, key, method string, options ...options.ClientOption) Client
	Workflow(seservice, workflowID, method string, options ...options.ClientOption) Client
	CancelInvocation(invocationId string)
	AttachInvocation(invocationId string, opts ...options.AttachOption) AttachFuture
	Awakeable(options ...options.AwakeableOption) AwakeableFuture
	ResolveAwakeable(id string, value any, options ...options.ResolveAwakeableOption)
	RejectAwakeable(id string, reason error)
	Select(futs ...Selectable) Selector
	Run(fn func(ctx RunContext) (any, error), output any, options ...options.RunOption) error
	RunAsync(fn func(ctx RunContext) (any, error), options ...options.RunOption) RunAsyncFuture

	// available on all keyed handlers
	Get(key string, output any, options ...options.GetOption) (bool, error)
	Keys() ([]string, error)
	Key() string

	// available on non-shared keyed handlers
	Set(key string, value any, options ...options.SetOption)
	Clear(key string)
	ClearAll()

	// available on workflow handlers
	Promise(name string, options ...options.PromiseOption) DurablePromise
}

type ctx struct {
	context.Context

	conn     io.ReadWriteCloser
	readChan chan readResult

	stateMachine *statemachine.StateMachine

	// Info about the invocation
	key          string
	request      Request
	isProcessing bool

	// Logging
	userLogContext atomic.Pointer[rcontext.LogContext]
	internalLogger *slog.Logger
	userLogger     *slog.Logger

	// Random
	rand rand.Rand

	// Run implementation
	runClosures           map[uint32]func() *pbinternal.VmProposeRunCompletionParameters
	runClosureCompletions chan *pbinternal.VmProposeRunCompletionParameters
}

var _ Context = (*ctx)(nil)

func newContext(inner context.Context, machine *statemachine.StateMachine, invocationInput *pbinternal.VmSysInputReturn_Input, conn io.ReadWriteCloser, attemptHeaders map[string][]string, dropReplayLogs bool, logHandler slog.Handler) *ctx {
	request := Request{
		ID:             invocationInput.GetInvocationId(),
		Headers:        make(map[string]string),
		AttemptHeaders: attemptHeaders,
		Body:           invocationInput.GetInput(),
	}
	for _, h := range invocationInput.GetHeaders() {
		request.Headers[h.GetKey()] = h.GetValue()
	}

	logHandler = logHandler.WithAttrs([]slog.Attr{slog.String("invocationID", invocationInput.GetInvocationId())})
	internalLogger := slog.New(log.NewRestateContextHandler(logHandler))
	inner = statemachine.WithLogger(inner, internalLogger)

	// will be cancelled when the http2 stream is cancelled
	// but NOT when we just doSuspend - just because we can't get completions doesn't mean we can't make
	// progress towards producing an output message
	ctx := &ctx{
		Context:               inner,
		conn:                  conn,
		readChan:              make(chan readResult),
		stateMachine:          machine,
		key:                   invocationInput.GetKey(),
		request:               request,
		internalLogger:        internalLogger,
		userLogContext:        atomic.Pointer[rcontext.LogContext]{},
		rand:                  rand.New([]byte(invocationInput.GetInvocationId())),
		userLogger:            nil,
		isProcessing:          false,
		runClosures:           make(map[uint32]func() *pbinternal.VmProposeRunCompletionParameters),
		runClosureCompletions: make(chan *pbinternal.VmProposeRunCompletionParameters, 10),
	}
	ctx.userLogger = slog.New(log.NewUserContextHandler(&ctx.userLogContext, dropReplayLogs, logHandler))

	// Prepare logger
	ctx.userLogContext.Store(&rcontext.LogContext{Source: rcontext.LogSourceUser, IsReplaying: true})

	// It might be we're already in processing phase
	ctx.checkStateTransition()

	return ctx
}

func (restateCtx *ctx) Log() *slog.Logger {
	return restateCtx.userLogger
}

func (restateCtx *ctx) Request() *Request {
	return &restateCtx.request
}

func (restateCtx *ctx) Rand() rand.Rand {
	return restateCtx.rand
}

func (restateCtx *ctx) Key() string {
	return restateCtx.key
}

func (restateCtx *ctx) checkStateTransition() {
	if restateCtx.isProcessing {
		return
	}
	// Check if we transitioned to processing
	processing, err := restateCtx.stateMachine.IsProcessing(restateCtx)
	if err != nil {
		panic(err)
	}
	if processing {
		restateCtx.userLogContext.Store(&rcontext.LogContext{Source: rcontext.LogSourceUser, IsReplaying: false})
		restateCtx.isProcessing = true
	}
}
