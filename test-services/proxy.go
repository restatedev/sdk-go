package main

import (
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/internal/options"
	"time"
)

type ProxyRequest struct {
	ServiceName      string  `json:"serviceName"`
	VirtualObjectKey *string `json:"virtualObjectKey,omitempty"`
	HandlerName      string  `json:"handlerName"`
	// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
	Message        []int   `json:"message"`
	IdempotencyKey *string `json:"idempotencyKey,omitempty"`
	DelayMillis    *uint64 `json:"delayMillis,omitempty"`
}

func (req *ProxyRequest) ToTarget(ctx restate.Context) restate.Client[[]byte, []byte] {
	if req.VirtualObjectKey != nil {
		return restate.WithRequestType[[]byte](restate.Object[[]byte](
			ctx,
			req.ServiceName,
			*req.VirtualObjectKey,
			req.HandlerName,
			restate.WithBinary))
	} else {
		return restate.WithRequestType[[]byte](restate.Service[[]byte](
			ctx,
			req.ServiceName,
			req.HandlerName,
			restate.WithBinary))
	}
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
					bytes, err := req.ToTarget(ctx).Request(input, opts...)
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
					if req.DelayMillis != nil {
						opts = append(opts, restate.WithDelay(time.Millisecond*time.Duration(*req.DelayMillis)))
					}
					req.ToTarget(ctx).Send(input, opts...)
					// TODO this should return the invocation id, when the API will be available
					return "invocationid", nil
				})).
			Handler("manyCalls", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, requests []ManyCallRequest) (restate.Void, error) {
					var toAwait []restate.Selectable

					for _, req := range requests {
						input := intArrayToByteArray(req.ProxyRequest.Message)
						if req.OneWayCall {
							var opts []options.SendOption
							if req.ProxyRequest.IdempotencyKey != nil {
								opts = append(opts, restate.WithIdempotencyKey(*req.ProxyRequest.IdempotencyKey))
							}
							if req.ProxyRequest.DelayMillis != nil {
								opts = append(opts, restate.WithDelay(time.Millisecond*time.Duration(*req.ProxyRequest.DelayMillis)))
							}
							req.ProxyRequest.ToTarget(ctx).Send(input, opts...)
						} else {
							var opts []options.RequestOption
							if req.ProxyRequest.IdempotencyKey != nil {
								opts = append(opts, restate.WithIdempotencyKey(*req.ProxyRequest.IdempotencyKey))
							}
							fut := req.ProxyRequest.ToTarget(ctx).RequestFuture(input, opts...)
							if req.AwaitAtTheEnd {
								toAwait = append(toAwait, fut)
							}
						}
					}

					selector := restate.Select(ctx, toAwait...)
					for selector.Remaining() {
						result := selector.Select()
						if _, err := result.(restate.ResponseFuture[[]byte]).Response(); err != nil {
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
