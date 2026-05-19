//go:build cgo

package loader

import (
	"fmt"
	"path/filepath"
	goplug "plugin"
	"sync"

	internalPlugin "helix/internal/plugin"
	"helix/sdk"
)

// Loader dynamically loads external Helix plugins compiled as .so files.
// Plugins must export:
//
//	var HelixPluginName string        — the plugin name
//	var HelixPlugin     sdk.Factory   — the factory function
type Loader struct {
	mu     sync.Mutex
	reg    *internalPlugin.Registry
	loaded map[string]struct{}
}

func New(reg *internalPlugin.Registry) *Loader {
	return &Loader{
		reg:    reg,
		loaded: make(map[string]struct{}),
	}
}

func (l *Loader) Load(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	p, err := goplug.Open(abs)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", abs, err)
	}

	nameSym, err := p.Lookup("HelixPluginName")
	if err != nil {
		return fmt.Errorf("plugin %s: missing HelixPluginName symbol: %w", abs, err)
	}
	namePtr, ok := nameSym.(*string)
	if !ok {
		return fmt.Errorf("plugin %s: HelixPluginName must be *string, got %T", abs, nameSym)
	}

	factorySym, err := p.Lookup("HelixPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s: missing HelixPlugin symbol: %w", abs, err)
	}
	factoryPtr, ok := factorySym.(*sdk.Factory)
	if !ok {
		return fmt.Errorf("plugin %s: HelixPlugin must be *sdk.Factory, got %T", abs, factorySym)
	}

	name := *namePtr
	factory := *factoryPtr
	l.reg.Register(name, func(cfg map[string]any) (internalPlugin.Plugin, error) {
		sdkPlugin, err := factory(cfg)
		if err != nil {
			return nil, err
		}
		return sdkPlugin, nil
	})

	l.loaded[abs] = struct{}{}
	return nil
}

func (l *Loader) IsLoaded(path string) bool {
	abs, _ := filepath.Abs(path)
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.loaded[abs]
	return ok
}
