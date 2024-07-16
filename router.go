package restate

import (
	"fmt"

	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
)

// Router interface
type Router interface {
	Name() string
	Type() internal.ServiceType
	// Set of handlers associated with this router
	Handlers() map[string]Handler
}

type serviceRouterOptions struct {
	defaultCodec encoding.PayloadCodec
}

type ServiceRouterOption interface {
	beforeServiceRouter(*serviceRouterOptions)
}

// ServiceRouter implements Router
type ServiceRouter struct {
	name     string
	handlers map[string]Handler
	options  serviceRouterOptions
}

var _ Router = &ServiceRouter{}

// NewServiceRouter creates a new ServiceRouter
func NewServiceRouter(name string, options ...ServiceRouterOption) *ServiceRouter {
	opts := serviceRouterOptions{}
	for _, opt := range options {
		opt.beforeServiceRouter(&opts)
	}
	if opts.defaultCodec == nil {
		opts.defaultCodec = encoding.JSONCodec{}
	}
	return &ServiceRouter{
		name:     name,
		handlers: make(map[string]Handler),
		options:  opts,
	}
}

func (r *ServiceRouter) Name() string {
	return r.name
}

// Handler registers a new handler by name
func (r *ServiceRouter) Handler(name string, handler ServiceHandler) *ServiceRouter {
	handler.getOptions().codec = encoding.MergeCodec(handler.getOptions().codec, r.options.defaultCodec)
	r.handlers[name] = handler
	return r
}

func (r *ServiceRouter) Handlers() map[string]Handler {
	return r.handlers
}

func (r *ServiceRouter) Type() internal.ServiceType {
	return internal.ServiceType_SERVICE
}

type objectRouterOptions struct {
	defaultCodec encoding.PayloadCodec
}

type ObjectRouterOption interface {
	beforeObjectRouter(*objectRouterOptions)
}

// ObjectRouter
type ObjectRouter struct {
	name     string
	handlers map[string]Handler
	options  objectRouterOptions
}

var _ Router = &ObjectRouter{}

func NewObjectRouter(name string, options ...ObjectRouterOption) *ObjectRouter {
	opts := objectRouterOptions{}
	for _, opt := range options {
		opt.beforeObjectRouter(&opts)
	}
	if opts.defaultCodec == nil {
		opts.defaultCodec = encoding.JSONCodec{}
	}
	return &ObjectRouter{
		name:     name,
		handlers: make(map[string]Handler),
		options:  opts,
	}
}

func (r *ObjectRouter) Name() string {
	return r.name
}

func (r *ObjectRouter) Handler(name string, handler ObjectHandler) *ObjectRouter {
	handler.getOptions().codec = encoding.MergeCodec(handler.getOptions().codec, r.options.defaultCodec)
	r.handlers[name] = handler
	return r
}

func (r *ObjectRouter) Handlers() map[string]Handler {
	return r.handlers
}

func (r *ObjectRouter) Type() internal.ServiceType {
	return internal.ServiceType_VIRTUAL_OBJECT
}
