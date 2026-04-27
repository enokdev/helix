package security

import (
	"time"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	helixsecurity "github.com/enokdev/helix/security"
	"github.com/enokdev/helix/starter/internal/starterutil"
)

const (
	secEnabledKey = "helix.starters.security.enabled"
	jwtSecretKey  = "security.jwt.secret"
	jwtExpiryKey  = "security.jwt.expiry"
)

// Starter auto-configures the security stack when security configuration is present.
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

// ConditionFromContainer evaluates the security starter activation after
// application components have been registered.
//
// Priority (highest to lowest):
//  1. helix.starters.security.enabled = false → inactive (absolute override)
//  2. helix.starters.security.enabled = true  → active (absolute override)
//  3. security.* key present in config        → active (config-key detection)
//  4. container holds a helixsecurity.Configurer → active (component marker)
//  5. otherwise                                → inactive
func (s *Starter) ConditionFromContainer(container *core.Container) bool {
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(secEnabledKey); ok {
			enabled, parsed := starterutil.ParseBool(value)
			if parsed {
				return enabled
			}
		}

		all := s.cfg.AllSettings()
		if _, ok := all["security"]; ok {
			return true
		}
	}

	if container == nil {
		return false
	}

	var cfg helixsecurity.Configurer
	return container.Resolve(&cfg) == nil
}

// Configure registers security components into the DI container.
func (s *Starter) Configure(container *core.Container) error {
	if container == nil {
		return nil
	}

	secret := ""
	expiry := 24 * time.Hour

	if s.cfg != nil {
		if v, ok := s.cfg.Lookup(jwtSecretKey); ok {
			if str, ok := v.(string); ok {
				secret = str
			}
		}
		if v, ok := s.cfg.Lookup(jwtExpiryKey); ok {
			if str, ok := v.(string); ok {
				if d, err := time.ParseDuration(str); err == nil && d > 0 {
					expiry = d
				}
			}
		}
	}

	if svc, err := helixsecurity.NewJWTService(secret, expiry); err == nil {
		_ = container.Register(svc)
	}
	return nil
}
