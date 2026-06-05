package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type wireResolverRepository struct{}

type wireResolverService struct {
	Repository *wireResolverRepository `inject:"true"`
}

type wireResolverLifecycle struct {
	started bool
}

type wireResolverGreeter interface {
	Greet() string
}

type wireResolverPrimaryGreeter struct{}

func (g *wireResolverPrimaryGreeter) Greet() string {
	return "primary"
}

type wireResolverFallbackGreeter struct{}

func (g *wireResolverFallbackGreeter) Greet() string {
	return "fallback"
}

func (l *wireResolverLifecycle) OnStart() error {
	l.started = true
	return nil
}

func (l *wireResolverLifecycle) OnStop(_ context.Context) error {
	l.started = false
	return nil
}

func TestWireResolver_Register_Resolve(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	service := &wireResolverService{Repository: &wireResolverRepository{}}

	if err := resolver.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var resolved *wireResolverService
	if err := resolver.Resolve(&resolved); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved != service {
		t.Fatalf("resolved instance = %p, want %p", resolved, service)
	}
	if resolved.Repository != service.Repository {
		t.Fatal("Resolve() should return the pre-wired instance without reinjecting fields")
	}
}

func TestWireResolver_ResolveNotFound(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()

	var resolved *wireResolverService
	err := resolver.Resolve(&resolved)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
	}
}

func TestWireResolverRegisterRejectsDuplicateConcreteType(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	first := &wireResolverService{Repository: &wireResolverRepository{}}
	second := &wireResolverService{Repository: &wireResolverRepository{}}

	if err := resolver.Register(first); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	err := resolver.Register(second)
	if !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("Register(second) error = %v, want ErrAlreadyRegistered", err)
	}

	var resolved *wireResolverService
	if err := resolver.Resolve(&resolved); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved != first {
		t.Fatalf("resolved = %p, want first registration %p", resolved, first)
	}
}

func TestWireResolverRegisterRejectsSharedExplicitInterface(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	greeterType := reflect.TypeOf((*wireResolverGreeter)(nil)).Elem()

	if err := resolver.Register(ComponentRegistration{
		Component: &wireResolverPrimaryGreeter{},
		ResolveAs: []reflect.Type{
			greeterType,
		},
	}); err != nil {
		t.Fatalf("Register(primary) error = %v", err)
	}

	err := resolver.Register(ComponentRegistration{
		Component: &wireResolverFallbackGreeter{},
		ResolveAs: []reflect.Type{
			greeterType,
		},
	})
	if !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("Register(fallback) error = %v, want ErrAlreadyRegistered", err)
	}
}

func TestWireResolverRegisterAllowsExplicitInterfacePriority(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	greeterType := reflect.TypeOf((*wireResolverGreeter)(nil)).Elem()
	primary := &wireResolverPrimaryGreeter{}

	if err := resolver.Register(ComponentRegistration{
		Component: primary,
		ResolveAs: []reflect.Type{
			greeterType,
		},
	}); err != nil {
		t.Fatalf("Register(primary) error = %v", err)
	}
	if err := resolver.Register(ComponentRegistration{
		Component: &wireResolverFallbackGreeter{},
		ExcludeFrom: []reflect.Type{
			greeterType,
		},
	}); err != nil {
		t.Fatalf("Register(fallback) error = %v", err)
	}

	var resolved wireResolverGreeter
	if err := resolver.Resolve(&resolved); err != nil {
		t.Fatalf("Resolve(greeter) error = %v", err)
	}
	if resolved != primary {
		t.Fatalf("resolved = %T, want primary", resolved)
	}
}

func TestWireResolverUnregister(t *testing.T) {
	resolver := NewWireResolver()
	component := &wireResolverService{}

	if err := resolver.Register(component); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := resolver.Unregister(component); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	var resolved *wireResolverService
	if err := resolver.Resolve(&resolved); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
	}
}

func TestWireResolverUnregisterComponentRegistration(t *testing.T) {
	resolver := NewWireResolver()
	registration := ComponentRegistration{Component: &wireResolverPrimaryGreeter{}}

	if err := resolver.Register(registration); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := resolver.Unregister(registration); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	var resolved *wireResolverPrimaryGreeter
	if err := resolver.Resolve(&resolved); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
	}
}

func TestWireResolver_LifecycleCandidates(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	lifecycle := &wireResolverLifecycle{}
	if err := resolver.Register(&wireResolverService{}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}
	if err := resolver.Register(lifecycle); err != nil {
		t.Fatalf("Register(lifecycle) error = %v", err)
	}

	candidates, err := resolver.LifecycleCandidates()
	if err != nil {
		t.Fatalf("LifecycleCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("LifecycleCandidates() length = %d, want 1", len(candidates))
	}
	if candidates[0].Instance != lifecycle {
		t.Fatalf("candidate instance = %p, want %p", candidates[0].Instance, lifecycle)
	}
}

func TestWireResolver_Graph(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	if err := resolver.Register(&wireResolverRepository{}); err != nil {
		t.Fatalf("Register(repository) error = %v", err)
	}
	if err := resolver.Register(&wireResolverService{}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}

	graph := resolver.Graph()
	wantNodes := map[string]bool{
		"*core.wireResolverRepository": false,
		"*core.wireResolverService":    false,
	}
	for _, node := range graph.Nodes {
		if _, ok := wantNodes[node]; ok {
			wantNodes[node] = true
		}
		if len(graph.Edges[node]) != 0 {
			t.Fatalf("Graph().Edges[%q] = %v, want empty", node, graph.Edges[node])
		}
	}
	for node, seen := range wantNodes {
		if !seen {
			t.Fatalf("Graph().Nodes missing %q: %v", node, graph.Nodes)
		}
	}
}

func TestWireResolverConcurrentAccessDoesNotRace(t *testing.T) {
	t.Parallel()

	resolver := NewWireResolver()
	service := &wireResolverService{Repository: &wireResolverRepository{}}
	lifecycle := &wireResolverLifecycle{}
	if err := resolver.Register(service); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}
	if err := resolver.Register(lifecycle); err != nil {
		t.Fatalf("Register(lifecycle) error = %v", err)
	}

	runConcurrently(t, 32, func() {
		var resolved *wireResolverService
		if err := resolver.Resolve(&resolved); err != nil {
			t.Errorf("Resolve() error = %v", err)
			return
		}
		if resolved != service {
			t.Errorf("resolved instance = %p, want %p", resolved, service)
		}
		if _, err := resolver.LifecycleCandidates(); err != nil {
			t.Errorf("LifecycleCandidates() error = %v", err)
		}
		_ = resolver.Graph()
	})
}
