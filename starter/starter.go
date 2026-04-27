package starter

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/enokdev/helix/core"
)

// Starter is the contract that auto-configuration modules must implement.
type Starter interface {
	// Condition reports whether this starter should be activated.
	Condition() bool
	// Configure registers components into the DI container.
	// It returns an error if configuration fails — for example when a required
	// dependency is missing or a component cannot be registered.
	Configure(*core.Container) error
}

// MarkerAwareStarter is an optional extension of Starter.
// Starters that implement it can evaluate their activation condition
// after application components have been registered in the container.
//
// Priority of evaluation (highest to lowest):
//  1. enabled: false in config  → never active
//  2. enabled: true in config   → always active
//  3. ConditionFromContainer    → active if container holds the expected marker
//  4. Condition                 → default structural detection (go.mod, config key)
type MarkerAwareStarter interface {
	Starter
	ConditionFromContainer(container *core.Container) bool
}

// ActivationReason describes why a starter was activated.
type ActivationReason string

const (
	// ReasonGoMod indicates the starter was activated because a dependency was
	// found in go.mod.
	ReasonGoMod ActivationReason = "go-mod"
	// ReasonConfigKey indicates the starter was activated because a matching
	// configuration key was present.
	ReasonConfigKey ActivationReason = "config-key"
	// ReasonComponentMarker indicates the starter was activated because a
	// component with the expected marker was found in the container.
	ReasonComponentMarker ActivationReason = "component-marker"
	// ReasonExplicit indicates the starter was activated because it was
	// explicitly enabled via configuration.
	ReasonExplicit ActivationReason = "explicit"
)

// Order controls the canonical activation sequence of starters.
type Order int

const (
	OrderConfig        Order = iota // 0 — configuration must be first
	OrderWeb                        // 1
	OrderData                       // 2
	OrderObservability              // 3
	OrderSecurity                   // 4
	OrderScheduling                 // 5

	orderMax = OrderScheduling
)

// Entry binds a Starter to its canonical order and a human-readable name.
type Entry struct {
	Name    string
	Order   Order
	Starter Starter
}

// Configure evaluates and activates each starter in canonical order.
// Validation errors are returned before any Configure call is made.
func Configure(container *core.Container, entries []Entry, opts ...Option) error {
	if container == nil {
		return fmt.Errorf("starter: configure: container is nil: %w", ErrInvalidStarter)
	}

	o := &options{logger: slog.Default()}
	for _, opt := range opts {
		opt(o)
	}

	// Validate all entries up-front so no starter is partially configured.
	for _, e := range entries {
		if err := validateEntry(e); err != nil {
			o.logger.Warn("starter validation failed",
				slog.String("starter", e.Name),
				slog.Int("order", int(e.Order)),
				slog.String("error", err.Error()),
			)
			return err
		}
	}

	// Stable sort preserves the relative order of entries sharing the same Order.
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	slices.SortStableFunc(sorted, func(a, b Entry) int {
		return int(a.Order) - int(b.Order)
	})

	for _, e := range sorted {
		// MarkerAwareStarters are deferred to the second pass (ConfigureMarkerAware)
		// which runs after application components are registered. Skip them here.
		if _, ok := e.Starter.(MarkerAwareStarter); ok {
			continue
		}
		active, reason := evaluateCondition(e.Starter, nil)
		o.logger.Debug("starter evaluated",
			slog.String("starter", e.Name),
			slog.Int("order", int(e.Order)),
			slog.Bool("active", active),
			slog.String("reason", string(reason)),
		)
		if active {
			if err := e.Starter.Configure(container); err != nil {
				return fmt.Errorf("starter: configure %q: %w", e.Name, err)
			}
		}
	}
	return nil
}

// ConfigureMarkerAware evaluates and activates marker-aware starters after
// application components have been registered in the container.
// It only processes starters that implement [MarkerAwareStarter]; others are skipped.
func ConfigureMarkerAware(container *core.Container, entries []Entry, opts ...Option) error {
	if container == nil {
		return fmt.Errorf("starter: configure marker-aware: container is nil: %w", ErrInvalidStarter)
	}

	o := &options{logger: slog.Default()}
	for _, opt := range opts {
		opt(o)
	}

	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	slices.SortStableFunc(sorted, func(a, b Entry) int {
		return int(a.Order) - int(b.Order)
	})

	for _, e := range sorted {
		mas, ok := e.Starter.(MarkerAwareStarter)
		if !ok {
			continue
		}
		active, reason := evaluateCondition(mas, container)
		o.logger.Debug("starter evaluated",
			slog.String("starter", e.Name),
			slog.Int("order", int(e.Order)),
			slog.Bool("active", active),
			slog.String("reason", string(reason)),
		)
		if active {
			if err := e.Starter.Configure(container); err != nil {
				return fmt.Errorf("starter: configure %q: %w", e.Name, err)
			}
		}
	}
	return nil
}

// evaluateCondition determines whether a starter should be activated and why.
// If container is non-nil and the starter implements [MarkerAwareStarter],
// ConditionFromContainer is used; otherwise Condition is used.
func evaluateCondition(s Starter, container *core.Container) (bool, ActivationReason) {
	if container != nil {
		if mas, ok := s.(MarkerAwareStarter); ok {
			active := mas.ConditionFromContainer(container)
			reason := ReasonComponentMarker
			if !active {
				reason = ReasonConfigKey
			}
			return active, reason
		}
	}
	return s.Condition(), ReasonConfigKey
}

func validateEntry(e Entry) error {
	if e.Name == "" {
		return fmt.Errorf("starter: configure %q: name is empty: %w", e.Name, ErrInvalidStarter)
	}
	if e.Starter == nil {
		return fmt.Errorf("starter: configure %q: starter is nil: %w", e.Name, ErrInvalidStarter)
	}
	if e.Order < OrderConfig || e.Order > orderMax {
		return fmt.Errorf("starter: configure %q: unknown order %d: %w", e.Name, e.Order, ErrInvalidStarter)
	}
	return nil
}
