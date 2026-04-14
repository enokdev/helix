package core

import "testing"

func TestNewComponentRegistration(t *testing.T) {
	t.Run("defaults to ScopeSingleton", func(t *testing.T) {
		reg := NewComponentRegistration(&struct{}{})
		if reg.Scope != ScopeSingleton {
			t.Errorf("expected ScopeSingleton, got %q", reg.Scope)
		}
	})

	t.Run("defaults Lazy to false", func(t *testing.T) {
		reg := NewComponentRegistration(&struct{}{})
		if reg.Lazy {
			t.Error("expected Lazy = false by default")
		}
	})

	t.Run("sets Component field", func(t *testing.T) {
		component := &struct{}{}
		reg := NewComponentRegistration(component)
		if reg.Component != component {
			t.Error("Component field not set correctly")
		}
	})
}
