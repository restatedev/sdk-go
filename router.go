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

// serviceDefinition stores a list of handlers under a named service
type serviceDefinition struct {
	name     string
	handlers map[string]Handler
	options  options.ServiceDefinitionOptions
	typ      internal.ServiceType
}

var _ ServiceDefinition = &serviceDefinition{}

// Name returns the name of the service described in this definition
func (r *serviceDefinition) Name() string {
	return r.name
}

// Handlers returns the list of handlers in this service definition
func (r *serviceDefinition) Handlers() map[string]Handler {
	return r.handlers
}

// Type returns the type of this service definition (Service or Virtual Object)
func (r *serviceDefinition) Type() internal.ServiceType {
	return r.typ
}

type service struct {
	serviceDefinition
}

// NewService creates a new named Service
func NewService(name string, opts ...options.ServiceDefinitionOption) *service {
	o := options.ServiceDefinitionOptions{}
	for _, opt := range opts {
		opt.BeforeServiceDefinition(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &service{
		serviceDefinition: serviceDefinition{
			name:     name,
			handlers: make(map[string]Handler),
			options:  o,
			typ:      internal.ServiceType_SERVICE,
		},
	}
}

// Handler registers a new Service handler by name
func (r *service) Handler(name string, handler ServiceHandler) *service {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}

type object struct {
	serviceDefinition
}

// NewObject creates a new named Virtual Object
func NewObject(name string, opts ...options.ServiceDefinitionOption) *object {
	o := options.ServiceDefinitionOptions{}
	for _, opt := range opts {
		opt.BeforeServiceDefinition(&o)
	}
	if o.DefaultCodec == nil {
		o.DefaultCodec = encoding.JSONCodec
	}
	return &object{
		serviceDefinition: serviceDefinition{
			name:     name,
			handlers: make(map[string]Handler),
			options:  o,
			typ:      internal.ServiceType_VIRTUAL_OBJECT,
		},
	}
}

// Handler registers a new Virtual Object handler by name
func (r *object) Handler(name string, handler ObjectHandler) *object {
	if handler.getOptions().Codec == nil {
		handler.getOptions().Codec = r.options.DefaultCodec
	}
	r.handlers[name] = handler
	return r
}
