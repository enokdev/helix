package observability

import "errors"

// ErrInvalidHealthIndicator is returned when a health indicator cannot be used.
var ErrInvalidHealthIndicator = errors.New("observability: invalid health indicator")

// ErrInvalidActuator is returned when actuator routes cannot be registered.
var ErrInvalidActuator = errors.New("observability: invalid actuator")
