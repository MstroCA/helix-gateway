//go:build !cgo

package loader

import (
	"errors"

	internalPlugin "helix/internal/plugin"
)

var errNoCGO = errors.New("external plugin loading requires CGO_ENABLED=1")

type Loader struct {
	reg *internalPlugin.Registry
}

func New(reg *internalPlugin.Registry) *Loader {
	return &Loader{reg: reg}
}

func (l *Loader) Load(_ string) error {
	return errNoCGO
}

func (l *Loader) IsLoaded(_ string) bool {
	return false
}
