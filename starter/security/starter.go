package security

import (
	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter/internal/starterutil"
)

const secEnabledKey = "helix.starters.security.enabled"

// Starter auto-configures the security stack when security configuration is present.
// NOTE: JWT/RBAC implementation is delegated to Epic 8. This starter currently
// registers a no-op lifecycle as a placeholder.
type Starter struct {
	cfg helixconfig.Loader
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the security starter should be activated.
func (s *Starter) Condition() bool {
	if s.cfg == nil {
		return false
	}

	if value, ok := s.cfg.Lookup(secEnabledKey); ok {
		enabled, parsed := starterutil.ParseBool(value)
		if parsed {
			return enabled
		}
	}

	// Auto-detect: activate if any top-level "security" key is present.
	all := s.cfg.AllSettings()
	_, ok := all["security"]
	return ok
}

// Configure registers security components into the DI container.
func (s *Starter) Configure(container *core.Container) {
	if container == nil {
		return
	}
	_ = container.Register(&securityLifecycle{})
}

// securityLifecycle is a no-op placeholder until Epic 8 provides the full
// JWT/RBAC implementation.
type securityLifecycle struct{}

func (l *securityLifecycle) OnStart() error { return nil }
func (l *securityLifecycle) OnStop() error  { return nil }
