package plugin

import "net/http"

// Plugin is the core extension point for Helix.
// Every built-in and third-party feature implements this interface.
type Plugin interface {
	Name() string
	Handler(next http.Handler) http.Handler
}

type pluginFunc struct {
	name string
	fn   func(http.Handler) http.Handler
}

func (p pluginFunc) Name() string                           { return p.name }
func (p pluginFunc) Handler(next http.Handler) http.Handler { return p.fn(next) }

// New wraps a middleware function as a named Plugin.
func New(name string, fn func(http.Handler) http.Handler) Plugin {
	return pluginFunc{name: name, fn: fn}
}

// Chain applies plugins in order — first listed is outermost (executed first).
func Chain(h http.Handler, plugins ...Plugin) http.Handler {
	for i := len(plugins) - 1; i >= 0; i-- {
		h = plugins[i].Handler(h)
	}
	return h
}

func WriteJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
