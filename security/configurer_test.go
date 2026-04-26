package security

import (
	"fmt"
	"testing"
	"time"

	"github.com/enokdev/helix/web"
)

// fakeContext implements web.Context for unit tests without a Fiber dependency.
type fakeContext struct {
	path   string
	header string
	locals map[string]any
}

func (c *fakeContext) Path() string             { return c.path }
func (c *fakeContext) OriginalURL() string      { return c.path }
func (c *fakeContext) Method() string           { return "GET" }
func (c *fakeContext) Param(string) string      { return "" }
func (c *fakeContext) Query(string) string      { return "" }
func (c *fakeContext) IP() string               { return "" }
func (c *fakeContext) Body() []byte             { return nil }
func (c *fakeContext) Status(int)               {}
func (c *fakeContext) SetHeader(string, string) {}
func (c *fakeContext) Send([]byte) error        { return nil }
func (c *fakeContext) JSON(any) error           { return nil }
func (c *fakeContext) Header(key string) string {
	if key == "Authorization" {
		return c.header
	}
	return ""
}

func (c *fakeContext) Locals(key string, value ...any) any {
	if len(value) > 0 {
		c.locals[key] = value[0]
		return value[0]
	}
	return c.locals[key]
}

var _ web.Context = (*fakeContext)(nil)

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected bool
	}{
		// Root
		{"/", "/", true},
		{"/", "", true},
		{"/", "/api", false},
		// Exact
		{"/api", "/api", true},
		{"/api", "/api/extra", false},
		// Single wildcard
		{"/api/*", "/api/users", true},
		{"/api/*/roles", "/api/users/roles", true},
		{"/api/*/roles", "/api/users/profile", false},
		// Double wildcard at end
		{"/api/**", "/api/users", true},
		{"/api/**", "/api/users/123/profile", true},
		{"/api/**", "/api", true},
		{"/**", "/anything/here", true},
		{"/actuator/**", "/actuator/health", true},
		{"/actuator/**", "/actuator/metrics/json", true},
		{"/actuator/**", "/api/health", false},
		// Double wildcard in the middle (consecutive ** handling)
		{"/api/**/health", "/api/v1/health", true},
		{"/api/**/health", "/api/v1/v2/health", true},
		{"/api/**/health", "/api/health", true},
		{"/api/**/health", "/api/v1/other", false},
		// URL-encoded paths are normalized before matching security rules.
		{"/api/**", "/api%2Fusers", true},
		{"/admin/**", "/adm%69n/secret", true},
		{"/api/*", "/api%2Fusers", true},
		{"/api/**", "/public%2Fusers", false},
		{"/api/**", "/api%2Gusers", false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tc.pattern, tc.path), func(t *testing.T) {
			result := matchesPattern(tc.pattern, tc.path)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestHTTPSecurity_Authenticated_BlocksEncodedPath(t *testing.T) {
	hs := NewHTTPSecurity(nil)
	hs.Route("/api/**").Authenticated()
	guard := hs.Build()

	ctx := &fakeContext{path: "/api%2Fusers", locals: map[string]any{}}
	if err := guard.CanActivate(ctx); err == nil {
		t.Fatal("expected encoded /api path to match protected rule and be denied")
	}
}

func TestHTTPSecurity_PermitAll(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"matched path allowed", "/public/assets/style.css", false},
		{"unmatched path allowed (no rule)", "/api/users", false},
	}

	hs := NewHTTPSecurity(nil)
	hs.Route("/public/**").PermitAll()
	guard := hs.Build()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &fakeContext{path: tc.path, locals: map[string]any{}}
			if err := guard.CanActivate(ctx); (err != nil) != tc.wantErr {
				t.Errorf("CanActivate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPSecurity_Authenticated(t *testing.T) {
	svc, _ := NewJWTService("secret", time.Hour)
	token, _ := svc.Generate(map[string]any{"id": "123"})

	tests := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{"no token → 401", "", true},
		{"valid token → pass", "Bearer " + token, false},
	}

	hs := NewHTTPSecurity(svc)
	hs.Route("/api/**").Authenticated()
	guard := hs.Build()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &fakeContext{path: "/api/users", header: tc.header, locals: map[string]any{}}
			if err := guard.CanActivate(ctx); (err != nil) != tc.wantErr {
				t.Errorf("CanActivate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPSecurity_Authenticated_NoJWTService(t *testing.T) {
	hs := NewHTTPSecurity(nil)
	hs.Route("/api/**").Authenticated()
	guard := hs.Build()

	ctx := &fakeContext{path: "/api/users", locals: map[string]any{}}
	if err := guard.CanActivate(ctx); err == nil {
		t.Error("expected error when no JWT service configured, got nil")
	}
}

func TestHTTPSecurity_HasRole(t *testing.T) {
	tests := []struct {
		name    string
		locals  map[string]any
		wantErr bool
	}{
		{
			"unauthenticated (no claims) → 401",
			map[string]any{},
			true,
		},
		{
			"authenticated but no roles → 403",
			map[string]any{"jwt_claims": map[string]any{}},
			true,
		},
		{
			"authenticated, wrong role → 403",
			map[string]any{"jwt_claims": map[string]any{"roles": []any{"USER"}}},
			true,
		},
		{
			"authenticated, correct role → pass",
			map[string]any{"jwt_claims": map[string]any{"roles": []any{"USER", "ADMIN"}}},
			false,
		},
	}

	hs := NewHTTPSecurity(nil)
	hs.Route("/admin/**").HasRole("ADMIN")
	guard := hs.Build()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &fakeContext{path: "/admin/dashboard", locals: tc.locals}
			if err := guard.CanActivate(ctx); (err != nil) != tc.wantErr {
				t.Errorf("CanActivate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPSecurity_HasRole_NoRoles(t *testing.T) {
	hs := NewHTTPSecurity(nil)
	hs.Route("/admin/**").HasRole() // zero roles — must not panic
	guard := hs.Build()

	ctx := &fakeContext{path: "/admin/dashboard", locals: map[string]any{}}
	if err := guard.CanActivate(ctx); err == nil {
		t.Error("expected error for HasRole with no roles, got nil")
	}
}

func TestHTTPSecurity_RulePriority(t *testing.T) {
	svc, _ := NewJWTService("secret", time.Hour)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// /api/public/** → PermitAll wins (defined first)
		{"public path → permit", "/api/public/info", false},
		// /api/** → Authenticated (no token)
		{"protected path no token → deny", "/api/users", true},
		// No matching rule
		{"unmatched path → permit", "/static/logo.png", false},
	}

	hs := NewHTTPSecurity(svc)
	hs.Route("/api/public/**").PermitAll()
	hs.Route("/api/**").Authenticated()
	guard := hs.Build()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &fakeContext{path: tc.path, locals: map[string]any{}}
			if err := guard.CanActivate(ctx); (err != nil) != tc.wantErr {
				t.Errorf("CanActivate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestHTTPSecurity_NoRuleMatches(t *testing.T) {
	hs := NewHTTPSecurity(nil)
	hs.Route("/api/**").Authenticated()
	guard := hs.Build()

	ctx := &fakeContext{path: "/public/index.html", locals: map[string]any{}}
	if err := guard.CanActivate(ctx); err != nil {
		t.Errorf("expected nil for unmatched path, got %v", err)
	}
}
