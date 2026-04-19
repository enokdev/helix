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
	Configure(*core.Container)
}

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
		active := e.Starter.Condition()
		o.logger.Debug("starter evaluated",
			slog.String("starter", e.Name),
			slog.Int("order", int(e.Order)),
			slog.Bool("active", active),
		)
		if active {
			e.Starter.Configure(container)
		}
	}
	return nil
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
