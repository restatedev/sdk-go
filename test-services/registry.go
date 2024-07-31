package main

import (
	"log"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

var REGISTRY = Registry{components: map[string]Component{}}

type Registry struct {
	components map[string]Component
}

type Component struct {
	Fqdn   string
	Binder func(endpoint *server.Restate)
}

func (r *Registry) Add(c Component) {
	r.components[c.Fqdn] = c
}

func (r *Registry) AddRouter(router restate.Router) {
	r.Add(Component{
		Fqdn:   router.Name(),
		Binder: func(e *server.Restate) { e.Bind(router) },
	})
}

func (r *Registry) RegisterAll(e *server.Restate) {
	for _, c := range r.components {
		c.Binder(e)
	}
}

func (r *Registry) Register(fqdns map[string]struct{}, e *server.Restate) {
	for fqdn := range fqdns {
		c, ok := r.components[fqdn]
		if !ok {
			log.Fatalf("unknown fqdn %s. Did you remember to import the test at app.ts?", fqdn)
		}
		c.Binder(e)
	}
}
