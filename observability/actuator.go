package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/enokdev/helix/web"
)

const (
	healthPath = "/actuator/health"
	infoPath   = "/actuator/info"
)

// RegisterActuatorRoutes registers the health and info actuator endpoints.
func RegisterActuatorRoutes(server web.HTTPServer, checker HealthChecker, info InfoProvider) error {
	if server == nil || isNilInterface(server) {
		return fmt.Errorf("observability: register actuator routes: server: %w", ErrInvalidActuator)
	}
	if checker == nil || isNilInterface(checker) {
		return fmt.Errorf("observability: register actuator routes: health checker: %w", ErrInvalidActuator)
	}
	if info == nil || isNilInterface(info) {
		return fmt.Errorf("observability: register actuator routes: info provider: %w", ErrInvalidActuator)
	}

	if err := server.RegisterRoute(http.MethodGet, healthPath, func(ctx web.Context) error {
		response := checker.Check(context.Background())
		if response.Status == StatusDown {
			ctx.Status(http.StatusServiceUnavailable)
		} else {
			ctx.Status(http.StatusOK)
		}
		return ctx.JSON(response)
	}); err != nil {
		return fmt.Errorf("observability: register actuator route %s: %w", healthPath, err)
	}

	if err := server.RegisterRoute(http.MethodGet, infoPath, func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(info.Info(context.Background()))
	}); err != nil {
		return fmt.Errorf("observability: register actuator route %s: %w", infoPath, err)
	}

	return nil
}
