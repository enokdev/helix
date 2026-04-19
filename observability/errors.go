package observability

import "errors"

// ErrInvalidHealthIndicator is returned when a health indicator cannot be used.
var ErrInvalidHealthIndicator = errors.New("observability: invalid health indicator")

// ErrInvalidActuator is returned when actuator routes cannot be registered.
var ErrInvalidActuator = errors.New("observability: invalid actuator")

// ErrInvalidMetrics is returned when metrics routes or observers cannot be configured.
var ErrInvalidMetrics = errors.New("observability: invalid metrics")

// ErrInvalidTracing is returned when tracing configuration is invalid.
var ErrInvalidTracing = errors.New("observability: invalid tracing")
