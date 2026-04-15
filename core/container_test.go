package core

import (
	"errors"
	"testing"
)

func TestContainer_Register(t *testing.T) {
	tests := []struct {
		name      string
		resolver  Resolver
		component any
		wantErr   error
	}{
		{
			name:      "nil resolver returns ErrUnresolvable",
			resolver:  nil,
			component: &struct{}{},
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "nil component returns ErrUnresolvable",
			resolver:  &stubResolver{},
			component: nil,
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "with resolver delegates to resolver",
			resolver:  &stubResolver{},
			component: &struct{}{},
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{resolver: tt.resolver}
			err := c.Register(tt.component)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainer_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		resolver Resolver
		target   any
		wantErr  error
	}{
		{
			name:     "nil resolver returns ErrUnresolvable",
			resolver: nil,
			target:   &struct{}{},
			wantErr:  ErrUnresolvable,
		},
		{
			name:     "nil target returns ErrUnresolvable",
			resolver: &stubResolver{},
			target:   nil,
			wantErr:  ErrUnresolvable,
		},
		{
			name:     "with resolver delegates to resolver",
			resolver: &stubResolver{},
			target:   &struct{}{},
			wantErr:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{resolver: tt.resolver}
			err := c.Resolve(tt.target)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewContainer(t *testing.T) {
	t.Run("no options returns non-nil container with nil resolver", func(t *testing.T) {
		c := NewContainer()
		if c == nil {
			t.Fatal("NewContainer() returned nil")
		}
		if c.resolver != nil {
			t.Error("expected nil resolver without options")
		}
	})

	t.Run("WithResolver sets resolver", func(t *testing.T) {
		r := &stubResolver{}
		c := NewContainer(WithResolver(r))
		if c.resolver != r {
			t.Error("WithResolver option not applied")
		}
	})

	t.Run("multiple options applied in order", func(t *testing.T) {
		r1 := &stubResolver{}
		r2 := &stubResolver{}
		c := NewContainer(WithResolver(r1), WithResolver(r2))
		if c.resolver != r2 {
			t.Error("last WithResolver should win")
		}
	})
}
