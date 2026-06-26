package main

import (
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/options"
)

type ProxyRequest struct {
	ServiceName      string  `json:"serviceName"`
	VirtualObjectKey *string `json:"virtualObjectKey,omitempty"`
	HandlerName      string  `json:"handlerName"`
	// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
	Message        []int   `json:"message"`
	IdempotencyKey *string `json:"idempotencyKey,omitempty"`
	DelayMillis    *uint64 `json:"delayMillis,omitempty"`
	Scope          *string `json:"scope,omitempty"`
	LimitKey       *string `json:"limitKey,omitempty"`
}

func (req *ProxyRequest) ToTarget(ctx restate.Context) (restate.Client[[]byte, []byte], error) {
	if req.VirtualObjectKey != nil {
		if req.Scope != nil {
			return nil, restate.TerminalErrorf("scoped object calls are not supported")
		}
		return restate.WithRequestType[[]byte](restate.Object[[]byte](
			ctx,
			req.ServiceName,
			*req.VirtualObjectKey,
			req.HandlerName,
			restate.WithBinary)), nil
	}
	clientOpts := []options.ClientOption{restate.WithBinary}
	if req.Scope != nil {
		clientOpts = append(clientOpts, restate.WithScope(*req.Scope))
	}
	return restate.WithRequestType[[]byte](restate.Service[[]byte](
		ctx,
		req.ServiceName,
		req.HandlerName,
		clientOpts...)), nil
}

type ManyCallRequest struct {
	ProxyRequest  ProxyRequest `json:"proxyRequest"`
	OneWayCall    bool         `json:"oneWayCall"`
	AwaitAtTheEnd bool         `json:"awaitAtTheEnd"`
}

func init() {
	REGISTRY.AddDefinition(
		restate.NewService("Proxy").
			Handler("call", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, req ProxyRequest) ([]int, error) {
					input := intArrayToByteArray(req.Message)
					var opts []options.RequestOption
					if req.IdempotencyKey != nil {
						opts = append(opts, restate.WithIdempotencyKey(*req.IdempotencyKey))
					}
					if req.LimitKey != nil {
						opts = append(opts, restate.WithLimitKey(*req.LimitKey))
					}
					target, err := req.ToTarget(ctx)
					if err != nil {
						return nil, err
					}
					bytes, err := target.Request(input, opts...)
					return byteArrayToIntArray(bytes), err
				})).
			Handler("oneWayCall", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, req ProxyRequest) (string, error) {
					input := intArrayToByteArray(req.Message)
					var opts []options.SendOption
					if req.IdempotencyKey != nil {
						opts = append(opts, restate.WithIdempotencyKey(*req.IdempotencyKey))
					}
					if req.LimitKey != nil {
						opts = append(opts, restate.WithLimitKey(*req.LimitKey))
					}
					if req.DelayMillis != nil {
						opts = append(opts, restate.WithDelay(time.Millisecond*time.Duration(*req.DelayMillis)))
					}
					target, err := req.ToTarget(ctx)
					if err != nil {
						return "", err
					}
					return target.Send(input, opts...).GetInvocationId(), nil
				})).
			Handler("manyCalls", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, requests []ManyCallRequest) (restate.Void, error) {
					var toAwait []restate.Future

					for _, req := range requests {
						input := intArrayToByteArray(req.ProxyRequest.Message)
						if req.OneWayCall {
							var opts []options.SendOption
							if req.ProxyRequest.IdempotencyKey != nil {
								opts = append(opts, restate.WithIdempotencyKey(*req.ProxyRequest.IdempotencyKey))
							}
							if req.ProxyRequest.LimitKey != nil {
								opts = append(opts, restate.WithLimitKey(*req.ProxyRequest.LimitKey))
							}
							if req.ProxyRequest.DelayMillis != nil {
								opts = append(opts, restate.WithDelay(time.Millisecond*time.Duration(*req.ProxyRequest.DelayMillis)))
							}
							target, err := req.ProxyRequest.ToTarget(ctx)
							if err != nil {
								return restate.Void{}, err
							}
							target.Send(input, opts...)
						} else {
							var opts []options.RequestOption
							if req.ProxyRequest.IdempotencyKey != nil {
								opts = append(opts, restate.WithIdempotencyKey(*req.ProxyRequest.IdempotencyKey))
							}
							if req.ProxyRequest.LimitKey != nil {
								opts = append(opts, restate.WithLimitKey(*req.ProxyRequest.LimitKey))
							}
							target, err := req.ProxyRequest.ToTarget(ctx)
							if err != nil {
								return restate.Void{}, err
							}
							fut := target.RequestFuture(input, opts...)
							if req.AwaitAtTheEnd {
								toAwait = append(toAwait, fut)
							}
						}
					}

					for fut, err := range restate.Wait(ctx, toAwait...) {
						if err != nil {
							return restate.Void{}, err
						}
						if _, err := fut.(restate.ResponseFuture[[]byte]).Response(); err != nil {
							return restate.Void{}, err
						}
					}

					return restate.Void{}, nil
				})))
}

func intArrayToByteArray(in []int) []byte {
	out := make([]byte, len(in))
	for idx, val := range in {
		out[idx] = byte(val)
	}
	return out
}

func byteArrayToIntArray(in []byte) []int {
	out := make([]int, len(in))
	for idx, val := range in {
		out[idx] = int(val)
	}
	return out
}
