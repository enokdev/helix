package core

import (
	"errors"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrCyclicDep", ErrCyclicDep},
		{"ErrUnresolvable", ErrUnresolvable},
	}
	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatalf("%s is nil", tt.name)
			}
			if tt.err.Error() == "" {
				t.Errorf("%s has empty message", tt.name)
			}
		})
	}
}

func TestCyclicDepError_Error(t *testing.T) {
	tests := []struct {
		name     string
		path     []string
		wantSubs []string
	}{
		{
			name:     "single step",
			path:     []string{"A"},
			wantSubs: []string{"A"},
		},
		{
			name:     "two step cycle",
			path:     []string{"A", "B", "A"},
			wantSubs: []string{"A", "→", "B"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &CyclicDepError{Path: tt.path}
			msg := e.Error()
			for _, sub := range tt.wantSubs {
				if !strings.Contains(msg, sub) {
					t.Errorf("Error() = %q, expected to contain %q", msg, sub)
				}
			}
		})
	}
}

func TestCyclicDepError_Unwrap(t *testing.T) {
	e := &CyclicDepError{Path: []string{"A", "B", "A"}}
	if !errors.Is(e, ErrCyclicDep) {
		t.Error("errors.Is(CyclicDepError, ErrCyclicDep) should be true")
	}
}
