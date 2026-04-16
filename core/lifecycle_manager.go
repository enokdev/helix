package core

import (
	"errors"
	"fmt"
	"reflect"
	"time"
)

var lifecycleType = reflect.TypeOf((*Lifecycle)(nil)).Elem()

type startedLifecycle struct {
	name     string
	instance Lifecycle
}

type lifecycleState struct {
	hasStarted bool
	started    []startedLifecycle
}

func (c *Container) resolveLifecycleComponents() ([]startedLifecycle, error) {
	lr, ok := c.resolver.(LifecycleResolver)
	if !ok {
		return nil, fmt.Errorf("core: start: resolver %T does not implement LifecycleResolver: %w", c.resolver, ErrUnresolvable)
	}

	candidates, err := lr.LifecycleCandidates()
	if err != nil {
		return nil, err
	}

	return orderLifecycleComponents(candidates, c.resolver.Graph())
}

func orderLifecycleComponents(
	candidates []LifecycleCandidate,
	graph DependencyGraph,
) ([]startedLifecycle, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	candidateMap := make(map[string]startedLifecycle, len(candidates))
	orderIndex := make(map[string]int, len(candidates))
	for i, c := range candidates {
		candidateMap[c.Name] = startedLifecycle{name: c.Name, instance: c.Instance}
		orderIndex[c.Name] = i
	}

	indegree := make(map[string]int, len(candidates))
	dependents := make(map[string][]string, len(candidates))
	for _, c := range candidates {
		indegree[c.Name] = 0
	}

	for owner := range candidateMap {
		for _, dependency := range graph.Edges[owner] {
			if _, ok := candidateMap[dependency]; !ok {
				continue
			}
			indegree[owner]++
			dependents[dependency] = append(dependents[dependency], owner)
		}
	}

	ordered := make([]startedLifecycle, 0, len(candidateMap))
	processed := make(map[string]bool, len(candidateMap))

	for len(ordered) < len(candidateMap) {
		nextName, found := nextLifecycleCandidate(indegree, processed, orderIndex)
		if !found {
			return nil, fmt.Errorf("core: order lifecycle components: %w", ErrCyclicDep)
		}

		processed[nextName] = true
		ordered = append(ordered, candidateMap[nextName])
		for _, dependent := range dependents[nextName] {
			indegree[dependent]--
		}
	}

	return ordered, nil
}

func nextLifecycleCandidate(indegree map[string]int, processed map[string]bool, orderIndex map[string]int) (string, bool) {
	nextName := ""
	nextIndex := len(orderIndex) + 1

	for name, degree := range indegree {
		if degree != 0 || processed[name] {
			continue
		}

		index, ok := orderIndex[name]
		if !ok {
			index = len(orderIndex)
		}

		if nextName == "" || index < nextIndex {
			nextName = name
			nextIndex = index
		}
	}

	return nextName, nextName != ""
}

func (c *Container) stopStartedComponents(started []startedLifecycle) error {
	if len(started) == 0 {
		return nil
	}

	deadline := time.Now().Add(c.shutdownTimeout)
	var errs []error

	for index := len(started) - 1; index >= 0; index-- {
		component := started[index]
		remaining := time.Until(deadline)
		if err := c.stopLifecycleComponent(component, remaining); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c *Container) stopLifecycleComponent(component startedLifecycle, remaining time.Duration) error {
	// Budget already exhausted: skip OnStop to avoid unbounded synchronous blocking.
	if remaining <= 0 {
		timeoutErr := fmt.Errorf("core: stop %s: %w", component.name, ErrShutdownTimeout)
		c.logger.Error("lifecycle stop skipped: shutdown budget exhausted", "component", component.name)
		return timeoutErr
	}

	result := make(chan error, 1)
	go func() {
		result <- component.instance.OnStop()
	}()

	timer := time.NewTimer(remaining)
	defer timer.Stop()

	select {
	case err := <-result:
		if err == nil {
			return nil
		}

		wrapped := fmt.Errorf("core: stop %s: %w", component.name, err)
		c.logger.Error("lifecycle stop failed", "component", component.name, "error", err)
		return wrapped
	case <-timer.C:
		timeoutErr := fmt.Errorf("core: stop %s: %w", component.name, ErrShutdownTimeout)
		c.logger.Error("lifecycle stop exceeded shutdown budget", "component", component.name, "timeout", remaining)
		return timeoutErr
	}
}
