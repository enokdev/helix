package starter

import "log/slog"

type options struct {
	logger *slog.Logger
}

// Option configures the starter orchestrator.
type Option func(*options)

// WithLogger sets the logger used to emit activation events.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}
