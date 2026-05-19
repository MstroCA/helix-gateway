package plugin

import (
	"fmt"
	"sort"
)

type Factory func(config map[string]any) (Plugin, error)

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

func (r *Registry) Register(name string, f Factory) {
	r.factories[name] = f
}

func (r *Registry) Build(name string, config map[string]any) (Plugin, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q: not registered", name)
	}
	return f(config)
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
