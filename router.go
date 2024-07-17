package restate

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
)

// Router is the set of methods implemented by both services and virtual objects
type Router interface {
	Name() string
	Type() internal.ServiceType
	// Set of handlers associated with this router
	Handlers() map[string]Handler
}

// ServiceRouter stores a list of handlers under a named Service
type ServiceRouter struct {
	name     string
	handlers map[string]Handler
	options  options.ServiceRouterOptions
}

var _ Router = &ServiceRouter{}

// NewServiceRouter creates a new named Service
func NewServiceRouter(name string, opts ...options.ServiceRouterOption) *ServiceRouter {
	o := options.ServiceRouterOptions{}
	for _, opt := range opts {
		opt.BeforeServiceRouter(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &ServiceRouter{
		name:     name,
		handlers: make(map[string]Handler),
		options:  o,
	}
}

// Name returns the name of this Service
func (r *ServiceRouter) Name() string {
	return r.name
}

// Handler registers a new Service handler by name
func (r *ServiceRouter) Handler(name string, handler ServiceHandler) *ServiceRouter {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}

// Handlers returns the list of handlers in this Service
func (r *ServiceRouter) Handlers() map[string]Handler {
	return r.handlers
}

// Type implements [Router] by returning [internal.ServiceType_SERVICE]
func (r *ServiceRouter) Type() internal.ServiceType {
	return internal.ServiceType_SERVICE
}

// ObjectRouter stores a list of handlers under a named Virtual Object
type ObjectRouter struct {
	name     string
	handlers map[string]Handler
	options  options.ObjectRouterOptions
}

var _ Router = &ObjectRouter{}

// NewObjectRouter creates a new named Virtual Object
func NewObjectRouter(name string, opts ...options.ObjectRouterOption) *ObjectRouter {
	o := options.ObjectRouterOptions{}
	for _, opt := range opts {
		opt.BeforeObjectRouter(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &ObjectRouter{
		name:     name,
		handlers: make(map[string]Handler),
		options:  o,
	}
}

// Name returns the name of this Virtual Object
func (r *ObjectRouter) Name() string {
	return r.name
}

// Handler registers a new Virtual Object handler by name
func (r *ObjectRouter) Handler(name string, handler ObjectHandler) *ObjectRouter {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}

// Handlers returns the list of handlers in this Virtual Object
func (r *ObjectRouter) Handlers() map[string]Handler {
	return r.handlers
}

// Type implements [Router] by returning [internal.ServiceType_VIRTUAL_OBJECT]
func (r *ObjectRouter) Type() internal.ServiceType {
	return internal.ServiceType_VIRTUAL_OBJECT
}
