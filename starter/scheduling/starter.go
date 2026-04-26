package scheduling

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/scheduler"
	"github.com/enokdev/helix/starter/internal/gomodutil"
	"github.com/enokdev/helix/starter/internal/starterutil"
)

const schedEnabledKey = "helix.starters.scheduling.enabled"

// Starter auto-configures the scheduling stack when robfig/cron is available.
type Starter struct {
	cfg helixconfig.Loader
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the scheduling starter should be activated.
func (s *Starter) Condition() bool {
	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(goModPath)
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
	sched := scheduler.NewScheduler()
	_ = container.Register(sched)
	_ = container.Register(newScheduledJobRegistrar(container, sched))
}

type scheduledJobRegistrar struct {
	container *core.Container
	sched     scheduler.Scheduler
	once      sync.Once
	startErr  error
}

var _ core.Lifecycle = (*scheduledJobRegistrar)(nil)

func newScheduledJobRegistrar(container *core.Container, sched scheduler.Scheduler) *scheduledJobRegistrar {
	return &scheduledJobRegistrar{container: container, sched: sched}
}

func (r *scheduledJobRegistrar) OnStart() error {
	r.once.Do(func() {
		r.startErr = r.doStart()
	})
	return r.startErr
}

func (r *scheduledJobRegistrar) doStart() error {
	providers, err := core.ResolveAll[scheduler.ScheduledJobProvider](r.container)
	if err != nil {
		return fmt.Errorf("scheduler: resolve scheduled job providers: %w", err)
	}

	for _, provider := range providers {
		for _, job := range provider.ScheduledJobs() {
			if job.Fn == nil {
				return fmt.Errorf("scheduler: job %q has nil Fn", job.Name)
			}
			fn := job.Fn
			allowConcurrent := job.AllowConcurrent
			if !job.AllowConcurrent {
				fn = scheduler.WrapSkipIfBusy(fn)
				allowConcurrent = true
			}
			if err := r.sched.Register(scheduler.Job{
				Name:            job.Name,
				Expr:            job.Expr,
				Fn:              fn,
				AllowConcurrent: allowConcurrent,
			}); err != nil {
				return fmt.Errorf("scheduler: register job %q: %w", job.Name, err)
			}
		}
	}
	return nil
}

func (r *scheduledJobRegistrar) OnStop() error {
	return nil
}
