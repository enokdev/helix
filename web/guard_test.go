package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enokdev/helix/web"
)

// okGuard is a Guard that always allows the request to proceed.
type okGuard struct{}

func (okGuard) CanActivate(_ web.Context) error { return nil }

// typedNilGuard returns a typed nil Guard to detect typed-nil defence.
func typedNilGuard() web.Guard {
	var g *okGuard
	return g
}

func TestApplyGlobalGuard_NilGuardReturnsError(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	err := web.ApplyGlobalGuard(server, nil)
	if err == nil {
		t.Fatal("ApplyGlobalGuard(nil) = nil, want error")
	}
}

func TestApplyGlobalGuard_TypedNilGuardReturnsError(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	err := web.ApplyGlobalGuard(server, typedNilGuard())
	if err == nil {
		t.Fatal("ApplyGlobalGuard(typed-nil) = nil, want error")
	}
}

func TestApplyGlobalGuard_ValidGuardRegistersSuccessfully(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	if err := web.ApplyGlobalGuard(server, okGuard{}); err != nil {
		t.Fatalf("ApplyGlobalGuard(valid guard) = %v, want nil", err)
	}
}

func TestApplyGlobalGuard_NilServerReturnsError(t *testing.T) {
	t.Parallel()

	err := web.ApplyGlobalGuard(nil, okGuard{})
	if err == nil {
		t.Fatal("ApplyGlobalGuard(nil server) = nil, want error")
	}
}

// blockingGuard always denies the request with 401.
type blockingGuard struct{}

func (blockingGuard) CanActivate(_ web.Context) error {
	return web.Unauthorized("blocked")
}

func TestApplyGlobalGuard_GuardBlocksAllRoutes(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	if err := web.ApplyGlobalGuard(server, blockingGuard{}); err != nil {
		t.Fatalf("ApplyGlobalGuard() = %v", err)
	}

	if err := server.RegisterRoute(http.MethodGet, "/test", func(ctx web.Context) error {
		return ctx.JSON(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute() = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/test", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d (guard should have blocked)", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestApplyGlobalGuard_ErrorMessageMentionsGuard(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	err := web.ApplyGlobalGuard(server, nil)
	if err == nil {
		t.Fatal("ApplyGlobalGuard(nil) = nil, want error")
	}
	if !errors.Is(err, web.ErrInvalidDirective) {
		t.Fatalf("ApplyGlobalGuard(nil) error = %v, want wrapping ErrInvalidDirective", err)
	}
}
