package core

import "testing"

// stubResolver is a minimal Resolver used only in option tests.
type stubResolver struct{}

func (s *stubResolver) Register(_ any) error    { return nil }
func (s *stubResolver) Resolve(_ any) error     { return nil }
func (s *stubResolver) Graph() DependencyGraph  { return DependencyGraph{} }

func TestWithResolver(t *testing.T) {
	r := &stubResolver{}
	c := NewContainer(WithResolver(r))
	if c.resolver != r {
		t.Error("WithResolver did not set the resolver on the container")
	}
}

func TestNewContainer_NoOptions(t *testing.T) {
	c := NewContainer()
	if c == nil {
		t.Fatal("NewContainer() returned nil")
	}
	if c.resolver != nil {
		t.Error("NewContainer() without options should have nil resolver")
	}
}
