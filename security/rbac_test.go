package security

import (
	"errors"
	"net/http"
	"testing"
)

// ─── RoleGuard tests ──────────────────────────────────────────────────────────

func TestRoleGuard_CanActivate(t *testing.T) {
	tests := []struct {
		name           string
		allowedRoles   []string
		claims         map[string]any // nil means no claims set in context
		wantErr        bool
		wantStatusCode int
	}{
		{
			name:         "user has the required role",
			allowedRoles: []string{"admin"},
			claims:       map[string]any{"roles": []any{"admin"}},
			wantErr:      false,
		},
		{
			name:         "user has one of the listed roles (admin,moderator) — has admin",
			allowedRoles: []string{"admin", "moderator"},
			claims:       map[string]any{"roles": []any{"admin"}},
			wantErr:      false,
		},
		{
			name:         "user has one of the listed roles (admin,moderator) — has moderator",
			allowedRoles: []string{"admin", "moderator"},
			claims:       map[string]any{"roles": []any{"moderator"}},
			wantErr:      false,
		},
		{
			name:         "user has multiple roles including required one",
			allowedRoles: []string{"admin"},
			claims:       map[string]any{"roles": []any{"user", "admin", "editor"}},
			wantErr:      false,
		},
		{
			name:         "roles claim is []string (programmatic construction)",
			allowedRoles: []string{"admin"},
			claims:       map[string]any{"roles": []string{"admin"}},
			wantErr:      false,
		},
		{
			name:           "user has incorrect role",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"roles": []any{"user"}},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "claims have no roles field",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"sub": "user-1"},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "roles field is not an array — string value",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"roles": "admin"},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "roles field is not an array — int value",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"roles": 42},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "roles array contains non-string elements",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"roles": []any{123, true}},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "empty roles array in claims",
			allowedRoles:   []string{"admin"},
			claims:         map[string]any{"roles": []any{}},
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "no claims in context — unauthenticated user",
			allowedRoles:   []string{"admin"},
			claims:         nil,
			wantErr:        true,
			wantStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard := NewRoleGuard(tt.allowedRoles...)
			ctx := newMockCtx("")

			// Simulate JWTGuard having stored claims (or not).
			if tt.claims != nil {
				ctx.Locals(claimsKey, tt.claims)
			}

			err := guard.CanActivate(ctx)

			if tt.wantErr {
				if err == nil {
					t.Fatal("CanActivate() expected error, got nil")
				}
				type statusCoder interface{ StatusCode() int }
				if sc, ok := err.(statusCoder); ok {
					if sc.StatusCode() != tt.wantStatusCode {
						t.Fatalf("StatusCode() = %d, want %d", sc.StatusCode(), tt.wantStatusCode)
					}
				} else {
					t.Fatalf("error does not implement statusCoder: %v", err)
				}
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("errors.Is(err, ErrForbidden) = false, want true")
				}
				return
			}

			if err != nil {
				t.Fatalf("CanActivate() unexpected error = %v", err)
			}
		})
	}
}

func TestNewRoleGuard_StoresAllowedRoles(t *testing.T) {
	guard := NewRoleGuard("admin", "moderator", "editor")
	if len(guard.allowedRoles) != 3 {
		t.Fatalf("expected 3 allowed roles, got %d", len(guard.allowedRoles))
	}
	expected := []string{"admin", "moderator", "editor"}
	for i, role := range expected {
		if guard.allowedRoles[i] != role {
			t.Fatalf("allowedRoles[%d] = %q, want %q", i, guard.allowedRoles[i], role)
		}
	}
}

func TestNewRoleGuard_PanicsWithNoRoles(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewRoleGuard() expected panic with zero roles, got none")
		}
	}()
	_ = NewRoleGuard()
}

func TestNewRoleGuard_DefensiveCopy(t *testing.T) {
	roles := []string{"admin"}
	guard := NewRoleGuard(roles...)
	roles[0] = "hacker"
	if guard.allowedRoles[0] != "admin" {
		t.Fatal("allowedRoles was mutated by caller — defensive copy missing")
	}
}

// ─── NewRoleGuardFactory tests ────────────────────────────────────────────────

func TestNewRoleGuardFactory(t *testing.T) {
	tests := []struct {
		name      string
		argument  string
		wantErr   bool
		wantRoles []string
	}{
		{name: "single role", argument: "admin", wantRoles: []string{"admin"}},
		{name: "multiple roles comma-separated", argument: "admin,moderator", wantRoles: []string{"admin", "moderator"}},
		{name: "roles with surrounding spaces", argument: " admin , moderator ", wantRoles: []string{"admin", "moderator"}},
		{name: "empty argument", argument: "", wantErr: true},
		{name: "only commas", argument: ",,,", wantErr: true},
	}

	factory := NewRoleGuardFactory()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guard, err := factory(tt.argument)
			if tt.wantErr {
				if err == nil {
					t.Fatal("factory() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("factory() unexpected error: %v", err)
			}
			rg, ok := guard.(*RoleGuard)
			if !ok {
				t.Fatalf("factory() returned %T, want *RoleGuard", guard)
			}
			if len(rg.allowedRoles) != len(tt.wantRoles) {
				t.Fatalf("allowedRoles length = %d, want %d", len(rg.allowedRoles), len(tt.wantRoles))
			}
			for i, r := range tt.wantRoles {
				if rg.allowedRoles[i] != r {
					t.Fatalf("allowedRoles[%d] = %q, want %q", i, rg.allowedRoles[i], r)
				}
			}
		})
	}
}
