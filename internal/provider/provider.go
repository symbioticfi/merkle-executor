// Package provider registers daemon-owned runtime data providers.
package provider

import (
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"gopkg.in/yaml.v3"
	"net/http"
)

type Deps struct {
	Chain     provider.Reader
	HTTP      *http.Client
	Log       logr.Logger
	ConfigDir string
}

type Factory func(raw yaml.Node, deps Deps) (provider.Provider, error)

// Registry is built during startup, then read by the single keeper loop.
type Registry struct{ factories map[string]Factory }

func NewRegistry() *Registry { return &Registry{factories: make(map[string]Factory)} }

func (r *Registry) Register(name string, factory Factory) error {
	if name == "" || factory == nil {
		return errors.New("provider name and factory required")
	}
	if _, exists := r.factories[name]; exists {
		return errors.Errorf("duplicate provider %q", name)
	}
	r.factories[name] = factory
	return nil
}

func (r *Registry) New(name string, raw yaml.Node, deps Deps) (provider.Provider, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, errors.Errorf("unknown provider %q", name)
	}
	return factory(raw, deps)
}
