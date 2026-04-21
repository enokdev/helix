package scheduling

import (
	"bytes"
	"os"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter/internal/starterutil"
)

const schedEnabledKey = "helix.starters.scheduling.enabled"

// Starter auto-configures the scheduling stack when robfig/cron is available.
// NOTE: The full scheduler implementation is delegated to Epic 9. This starter
// currently registers a no-op lifecycle as a placeholder.
type Starter struct {
	cfg helixconfig.Loader
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the scheduling starter should be activated.
func (s *Starter) Condition() bool {
	data, err := os.ReadFile("go.mod")
	if err != nil || !bytes.Contains(data, []byte("robfig/cron")) {
		return false
	}

	if s.cfg == nil {
		return true
	}

	if value, ok := s.cfg.Lookup(schedEnabledKey); ok {
		enabled, parsed := starterutil.ParseBool(value)
		if parsed {
			return enabled
		}
	}

	return true
}

// Configure registers scheduling components into the DI container.
func (s *Starter) Configure(container *core.Container) {
	if container == nil {
		return
	}
	_ = container.Register(&schedulingLifecycle{})
}

// schedulingLifecycle is a no-op placeholder until Epic 9 provides the full
// cron scheduler implementation.
type schedulingLifecycle struct{}

func (l *schedulingLifecycle) OnStart() error { return nil }
func (l *schedulingLifecycle) OnStop() error  { return nil }
