package web

import "log/slog"

const requestLoggerLocalKey = "_helix_logger"

func loggerFromContext(ctx Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Locals(requestLoggerLocalKey).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}

func loggerFromServer(server HTTPServer) *slog.Logger {
	if provider, ok := server.(interface{ logger() *slog.Logger }); ok {
		if logger := provider.logger(); logger != nil {
			return logger
		}
	}
	return slog.Default()
}
