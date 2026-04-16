package config

import "errors"

// Sentinel errors returned by the configuration loader.
var (
	ErrConfigNotFound = errors.New("helix: config not found")
	ErrInvalidConfig  = errors.New("helix: invalid config")
)
