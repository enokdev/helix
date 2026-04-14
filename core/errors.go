
package core

import (
	"errors"
	"strings"
)

// Sentinel errors returned by the Helix DI container.
var (
	ErrNotFound     = errors.New("helix: not found")
	ErrCyclicDep    = errors.New("helix: cyclic dependency")
	ErrUnresolvable = errors.New("helix: cannot resolve component")
)

// CyclicDepError is returned when a cyclic dependency is detected.
// It wraps ErrCyclicDep and includes the full dependency path.
type CyclicDepError struct {
	Path []string
}

func (e *CyclicDepError) Error() string {
	return "helix: cyclic dependency: " + strings.Join(e.Path, " → ")
}

// Unwrap allows errors.Is(err, ErrCyclicDep) to return true.
func (e *CyclicDepError) Unwrap() error {
	return ErrCyclicDep
}
