package sdk

import "net/http"

// Plugin is the interface all Helix plugins must implement.
// External plugins compiled as .so files export this via the HelixPlugin symbol.
type Plugin interface {
	Name() string
	Handler(next http.Handler) http.Handler
}

// Factory creates a configured Plugin instance from an option map.
// External .so files export a variable of type Factory named "HelixPlugin",
// and a string variable named "HelixPluginName" containing the plugin's name.
type Factory func(config map[string]any) (Plugin, error)
