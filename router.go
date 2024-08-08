package restate

import (
	"github.com/restatedev/sdk-go/encoding"
	"github.com/restatedev/sdk-go/internal"
	"github.com/restatedev/sdk-go/internal/options"
)

// ServiceDefinition is the set of methods implemented by both services and virtual objects
type ServiceDefinition interface {
	Name() string
	Type() internal.ServiceType
	// Set of handlers associated with this service definition
	Handlers() map[string]Handler
}

// service stores a list of handlers under a named Service
type service struct {
	name     string
	handlers map[string]Handler
	options  options.ServiceOptions
}

var _ ServiceDefinition = &service{}

// NewService creates a new named Service
func NewService(name string, opts ...options.ServiceOption) *service {
	o := options.ServiceOptions{}
	for _, opt := range opts {
		opt.BeforeService(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &service{
		name:     name,
		handlers: make(map[string]Handler),
		options:  o,
	}
}

// Name returns the name of this Service
func (r *service) Name() string {
	return r.name
}

// Handler registers a new Service handler by name
func (r *service) Handler(name string, handler ServiceHandler) *service {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}

// Handlers returns the list of handlers in this Service
func (r *service) Handlers() map[string]Handler {
	return r.handlers
}

// Type implements [ServiceDefinition] by returning [internal.ServiceType_SERVICE]
func (r *service) Type() internal.ServiceType {
	return internal.ServiceType_SERVICE
}

// object stores a list of handlers under a named Virtual Object
type object struct {
	name     string
	handlers map[string]Handler
	options  options.ObjectOptions
}

var _ ServiceDefinition = &object{}

// NewObject creates a new named Virtual Object
func NewObject(name string, opts ...options.ObjectOption) *object {
	o := options.ObjectOptions{}
	for _, opt := range opts {
		opt.BeforeObject(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &object{
		name:     name,
		handlers: make(map[string]Handler),
		options:  o,
	}
}

// Name returns the name of this Virtual Object
func (r *object) Name() string {
	return r.name
}

// Handler registers a new Virtual Object handler by name
func (r *object) Handler(name string, handler ObjectHandler) *object {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}

// Handlers returns the list of handlers in this Virtual Object
func (r *object) Handlers() map[string]Handler {
	return r.handlers
}

// Type implements [ServiceDefinition] by returning [internal.ServiceType_VIRTUAL_OBJECT]
func (r *object) Type() internal.ServiceType {
	return internal.ServiceType_VIRTUAL_OBJECT
}
