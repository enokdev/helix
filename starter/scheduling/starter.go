package scheduling

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	cfg           helixconfig.Loader
	mu            sync.Mutex
	configuredFor *core.Container
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the scheduling starter should be activated.
func (s *Starter) Condition() bool {
	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		slog.Debug("scheduling starter: go.mod not found", "error", err)
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

// ConditionFromContainer evaluates the scheduling starter activation after
// application components have been registered.
//
// Priority (highest to lowest):
//  1. helix.starters.scheduling.enabled = false → inactive (absolute override)
//  2. helix.starters.scheduling.enabled = true  → active (absolute override)
//  3. robfig/cron absent from go.mod            → inactive (missing runtime dependency)
//  4. container holds a scheduler.ScheduledJobProvider → active (component marker)
//  5. otherwise → inactive
func (s *Starter) ConditionFromContainer(container *core.Container) bool {
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(schedEnabledKey); ok {
			enabled, parsed := starterutil.ParseBool(value)
			if parsed {
				return enabled
			}
		}
	}

	if container == nil {
		return false
	}

	// Keep the same go.mod guard as Condition() for consistency.
	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(goModPath)
	if err != nil || !bytes.Contains(data, []byte("robfig/cron")) {
		return false
	}

	providers, err := core.ResolveAll[scheduler.ScheduledJobProvider](container)
	return err == nil && len(providers) > 0
}

// Configure registers scheduling components into the DI container.
func (s *Starter) Configure(container *core.Container) error {
	if container == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configuredFor == container {
		return nil
	}
	sched := scheduler.NewScheduler()
	if err := container.Register(sched); err != nil {
		return fmt.Errorf("scheduling starter: register scheduler: %w", err)
	}
	if err := container.Register(newScheduledJobRegistrar(container, sched)); err != nil {
		return fmt.Errorf("scheduling starter: register scheduled job registrar: %w", err)
	}
	s.configuredFor = container
	return nil
}

type scheduledJobRegistrar struct {
	container *core.Container
	sched     scheduler.Scheduler
	mu        sync.Mutex
	started   bool
	startErr  error
}

var _ core.Lifecycle = (*scheduledJobRegistrar)(nil)

type jobUnregisterer interface {
	Unregister(name string) error
}

func newScheduledJobRegistrar(container *core.Container, sched scheduler.Scheduler) *scheduledJobRegistrar {
	return &scheduledJobRegistrar{container: container, sched: sched}
}

func (r *scheduledJobRegistrar) OnStart() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}
	r.startErr = r.doStart()
	if r.startErr == nil {
		r.started = true
	}
	return r.startErr
}

func (r *scheduledJobRegistrar) doStart() (doErr error) {
	providers, err := core.ResolveAll[scheduler.ScheduledJobProvider](r.container)
	if err != nil {
		return fmt.Errorf("scheduler: resolve scheduled job providers: %w", err)
	}

	registered := make([]string, 0)
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := fmt.Errorf("scheduler: job provider panicked: %v", recovered)
			if rollbackErr := r.rollbackRegisteredJobs(registered); rollbackErr != nil {
				doErr = errors.Join(panicErr, rollbackErr)
				return
			}
			doErr = panicErr
		}
	}()
	for _, provider := range providers {
		for _, job := range provider.ScheduledJobs() {
			if err := r.sched.Register(job); err != nil {
				registerErr := fmt.Errorf("scheduler: register job %q: %w", job.Name, err)
				if rollbackErr := r.rollbackRegisteredJobs(registered); rollbackErr != nil {
					return errors.Join(registerErr, rollbackErr)
				}
				return registerErr
			}
			registered = append(registered, job.Name)
		}
	}
	return nil
}

func (r *scheduledJobRegistrar) rollbackRegisteredJobs(names []string) error {
	unregisterer, ok := r.sched.(jobUnregisterer)
	if !ok {
		return fmt.Errorf("scheduler: rollback unavailable: %T cannot unregister jobs", r.sched)
	}

	var rollbackErr error
	for i := len(names) - 1; i >= 0; i-- {
		if err := unregisterer.Unregister(names[i]); err != nil {
			if errors.Is(err, scheduler.ErrJobNotFound) {
				continue
			}
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("scheduler: rollback job %q: %w", names[i], err))
		}
	}
	return rollbackErr
}

func (r *scheduledJobRegistrar) OnStop(_ context.Context) error {
	return nil
}
