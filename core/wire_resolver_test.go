package core

import (
	"errors"
	"testing"
)

type wireResolverRepository struct{}

type wireResolverService struct {
	Repository *wireResolverRepository `inject:"true"`
}

type wireResolverLifecycle struct {
	started bool
}

func (l *wireResolverLifecycle) OnStart() error {
	l.started = true
	return nil
}

func (l *wireResolverLifecycle) OnStop() error {
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
