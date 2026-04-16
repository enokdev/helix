package helix

import "errors"

var (
	// ErrInvalidComponent is returned when a value cannot be registered as a Helix component.
	ErrInvalidComponent = errors.New("helix: invalid component")
	// ErrScanRequiresComponents is returned when source scan finds marker types
	// that cannot be instantiated by the runtime bootstrap.
	ErrScanRequiresComponents = errors.New("helix: scan requires runtime components")
)
