package observability

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/enokdev/helix/core"
)

// Status is the aggregate or component health state.
type Status string

const (
	// StatusUp means a component is healthy.
	StatusUp Status = "UP"
	// StatusDown means a component is unhealthy.
	StatusDown Status = "DOWN"
)

// ComponentHealth describes the health of one named component.
type ComponentHealth struct {
	Status  Status         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// HealthResponse is the JSON response returned by /actuator/health.
type HealthResponse struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// HealthIndicator is implemented by components that contribute to health.
type HealthIndicator interface {
	Name() string
	Health(context.Context) ComponentHealth
}

// HealthChecker evaluates the application health.
type HealthChecker interface {
	Check(context.Context) HealthResponse
}

type namedHealthIndicator struct {
	name      string
	indicator HealthIndicator
}

// CompositeHealthChecker combines several indicators into one health response.
type CompositeHealthChecker struct {
	indicators []namedHealthIndicator
}

// NewCompositeHealthChecker creates a deterministic health checker.
func NewCompositeHealthChecker(indicators ...HealthIndicator) (*CompositeHealthChecker, error) {
	seen := make(map[string]struct{}, len(indicators))
	named := make([]namedHealthIndicator, 0, len(indicators))

	for _, indicator := range indicators {
		if isNilHealthIndicator(indicator) {
			return nil, fmt.Errorf("observability: validate health indicator: %w", ErrInvalidHealthIndicator)
		}
		name := strings.TrimSpace(indicator.Name())
		if name == "" {
			return nil, fmt.Errorf("observability: validate health indicator name: %w", ErrInvalidHealthIndicator)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("observability: validate health indicator %q duplicate: %w", name, ErrInvalidHealthIndicator)
		}
		seen[name] = struct{}{}
		named = append(named, namedHealthIndicator{
			name:      name,
			indicator: indicator,
		})
	}

	return &CompositeHealthChecker{indicators: named}, nil
}

// HealthCheckerFromContainer builds a checker from registered HealthIndicator components.
func HealthCheckerFromContainer(container *core.Container) (*CompositeHealthChecker, error) {
	indicators, err := core.ResolveAll[HealthIndicator](container)
	if err != nil {
		return nil, fmt.Errorf("observability: resolve health indicators: %w", err)
	}
	checker, err := NewCompositeHealthChecker(indicators...)
	if err != nil {
		return nil, fmt.Errorf("observability: create health checker: %w", err)
	}
	return checker, nil
}

// Check returns the aggregate health response.
func (c *CompositeHealthChecker) Check(ctx context.Context) HealthResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	response := HealthResponse{Status: StatusUp}
	components := make(map[string]ComponentHealth)

	for _, indicator := range c.indicators {
		component := normalizeComponentHealth(indicator.indicator.Health(ctx))
		if component.Status == StatusDown {
			response.Status = StatusDown
		}
		if includeComponent(component) {
			components[indicator.name] = component
		}
	}

	if len(components) > 0 {
		response.Components = components
	}
	return response
}

func normalizeComponentHealth(component ComponentHealth) ComponentHealth {
	if component.Status == "" {
		component.Status = StatusUp
	}
	if component.Error != "" {
		component.Status = StatusDown
	}
	if component.Status == StatusDown && component.Error == "" {
		component.Error = "component reported unhealthy status"
	}
	return component
}

func includeComponent(component ComponentHealth) bool {
	return component.Status == StatusDown || component.Error != "" || len(component.Details) > 0
}

func isNilHealthIndicator(indicator HealthIndicator) bool {
	if indicator == nil {
		return true
	}
	value := reflect.ValueOf(indicator)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
