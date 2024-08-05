package main

import (
	restate "github.com/restatedev/sdk-go"
)

type ProxyRequest struct {
	ServiceName      string  `json:"serviceName"`
	VirtualObjectKey *string `json:"virtualObjectKey,omitempty"`
	HandlerName      string  `json:"handlerName"`
	// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
	Message []int `json:"message"`
}

type ManyCallRequest struct {
	ProxyRequest  ProxyRequest `json:"proxyRequest"`
	OneWayCall    bool         `json:"oneWayCall"`
	AwaitAtTheEnd bool         `json:"awaitAtTheEnd"`
}

func RegisterProxy() {
	REGISTRY.AddRouter(
		restate.NewServiceRouter("Proxy").
			Handler("call", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, req ProxyRequest) ([]int, error) {
					input := intArrayToByteArray(req.Message)
					if req.VirtualObjectKey != nil {
						var output []byte
						err := ctx.Object(
							req.ServiceName,
							*req.VirtualObjectKey,
							req.HandlerName,
							restate.WithBinary).
							Request(input, &output)
						if err != nil {
							return nil, err
						}
						return byteArrayToIntArray(output), nil
					} else {
						var output []byte
						err := ctx.Service(
							req.ServiceName,
							req.HandlerName,
							restate.WithBinary).
							Request(input, &output)
						if err != nil {
							return nil, err
						}
						return byteArrayToIntArray(output), nil
					}
				})).
			Handler("oneWayCall", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, req ProxyRequest) (restate.Void, error) {
					input := intArrayToByteArray(req.Message)
					if req.VirtualObjectKey != nil {
						err := ctx.Object(
							req.ServiceName,
							*req.VirtualObjectKey,
							req.HandlerName,
							restate.WithBinary).
							Send(input, 0)
						return restate.Void{}, err
					} else {
						err := ctx.Service(
							req.ServiceName,
							req.HandlerName,
							restate.WithBinary).
							Send(input, 0)
						return restate.Void{}, err
					}
				})).
			Handler("manyCalls", restate.NewServiceHandler(
				// We need to use []int because Golang takes the opinionated choice of treating []byte as Base64
				func(ctx restate.Context, requests []ManyCallRequest) (restate.Void, error) {
					var toAwait []restate.ResponseFuture

					for _, req := range requests {
						input := intArrayToByteArray(req.ProxyRequest.Message)
						if req.OneWayCall {
							if req.ProxyRequest.VirtualObjectKey != nil {
								err := ctx.Object(
									req.ProxyRequest.ServiceName,
									*req.ProxyRequest.VirtualObjectKey,
									req.ProxyRequest.HandlerName,
									restate.WithBinary).
									Send(input, 0)
								return restate.Void{}, err
							} else {
								err := ctx.Service(
									req.ProxyRequest.ServiceName,
									req.ProxyRequest.HandlerName,
									restate.WithBinary).
									Send(input, 0)
								return restate.Void{}, err
							}
						} else {
							if req.ProxyRequest.VirtualObjectKey != nil {
								fut, err := ctx.Object(
									req.ProxyRequest.ServiceName,
									*req.ProxyRequest.VirtualObjectKey,
									req.ProxyRequest.HandlerName,
									restate.WithBinary).
									RequestFuture(input)
								if err != nil {
									return restate.Void{}, err
								}
								if req.AwaitAtTheEnd {
									toAwait = append(toAwait, fut)
								}
							} else {
								fut, err := ctx.Service(
									req.ProxyRequest.ServiceName,
									req.ProxyRequest.HandlerName,
									restate.WithBinary).
									RequestFuture(input)
								if err != nil {
									return restate.Void{}, err
								}
								if req.AwaitAtTheEnd {
									toAwait = append(toAwait, fut)
								}
							}
						}
					}

					// TODO replace this with select
					for _, fut := range toAwait {
						var output []byte
						err := fut.Response(&output)
						if err != nil {
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
