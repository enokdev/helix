package core

import "testing"

// stubResolver is a minimal Resolver used only in option tests.
type stubResolver struct{}

func (s *stubResolver) Register(_ any) error   { return nil }
func (s *stubResolver) Resolve(_ any) error    { return nil }
func (s *stubResolver) Graph() DependencyGraph { return DependencyGraph{} }

func TestWithResolver(t *testing.T) {
	r := &stubResolver{}
	c := NewContainer(WithResolver(r))
	if c.resolver != r {
		t.Error("WithResolver did not set the resolver on the container")
	}
}

func TestWithResolver_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("WithResolver(nil) should panic")
		}
	}()
	WithResolver(nil)
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

type optionValueConsumer struct {
	Port int `value:"server.port"`
}

func TestWithValueLookup(t *testing.T) {
	tests := []struct {
		name string
		opts func(func(string) (any, bool)) []Option
	}{
		{
			name: "lookup after resolver",
			opts: func(lookup func(string) (any, bool)) []Option {
				return []Option{WithResolver(NewReflectResolver()), WithValueLookup(lookup)}
			},
		},
		{
			name: "lookup before resolver",
			opts: func(lookup func(string) (any, bool)) []Option {
				return []Option{WithValueLookup(lookup), WithResolver(NewReflectResolver())}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (any, bool) {
				if key != "server.port" {
					return nil, false
				}
				return "9090", true
			}
			container := NewContainer(tt.opts(lookup)...)
			if err := container.Register(&optionValueConsumer{}); err != nil {
				t.Fatalf("Register() error = %v", err)
			}

			var consumer *optionValueConsumer
			if err := container.Resolve(&consumer); err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if consumer.Port != 9090 {
				t.Fatalf("consumer.Port = %d, want 9090", consumer.Port)
			}
		})
	}
}

func TestWithValueLookup_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("WithValueLookup(nil) should panic")
		}
	}()
	WithValueLookup(nil)
}
