package security

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/enokdev/helix/web"
)

// ─── minimal mock Context ─────────────────────────────────────────────────────

type mockCtx struct {
	headers map[string]string
	locals  map[string]any
}

func newMockCtx(authHeader string) *mockCtx {
	headers := map[string]string{}
	if authHeader != "" {
		headers["Authorization"] = authHeader
	}
	return &mockCtx{headers: headers, locals: map[string]any{}}
}

func (m *mockCtx) Method() string           { return "GET" }
func (m *mockCtx) Path() string             { return "/" }
func (m *mockCtx) OriginalURL() string      { return "/" }
func (m *mockCtx) Param(_ string) string    { return "" }
func (m *mockCtx) Query(_ string) string    { return "" }
func (m *mockCtx) Header(key string) string { return m.headers[key] }
func (m *mockCtx) IP() string               { return "" }
func (m *mockCtx) Body() []byte             { return nil }
func (m *mockCtx) Status(_ int)             {}
func (m *mockCtx) SetHeader(_, _ string)    {}
func (m *mockCtx) Send(_ []byte) error      { return nil }
func (m *mockCtx) JSON(_ any) error         { return nil }
func (m *mockCtx) Context() context.Context { return context.Background() }
func (m *mockCtx) Locals(key string, value ...any) any {
	if len(value) > 0 {
		m.locals[key] = value[0]
		return value[0]
	}
	return m.locals[key]
}

// Compile-time assertion: mockCtx implements web.Context.
var _ web.Context = (*mockCtx)(nil)

// ─── JWTGuard tests ───────────────────────────────────────────────────────────

func newTestGuard() (*JWTGuard, *JWTService) {
	svc, err := NewJWTService(testSecret, time.Hour)
	if err != nil {
		panic(err)
	}
	return NewJWTGuard(svc), svc
}

func TestJWTGuard_CanActivate(t *testing.T) {
	guard, svc := newTestGuard()
	validToken, _ := svc.Generate(map[string]any{"sub": "user-1", "role": "admin"})

	tests := []struct {
		name       string
		authHeader string
		wantErr    bool
		wantStatus int
		wantClaims bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer " + validToken,
			wantClaims: true,
		},
		{
			name:       "missing header",
			authHeader: "",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "basic auth scheme",
			authHeader: "Basic dXNlcjpwYXNz",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "bearer prefix only",
			authHeader: "Bearer ",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer not.a.valid.token",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "tampered token",
			authHeader: "Bearer " + validToken + "x",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockCtx(tt.authHeader)
			err := guard.CanActivate(ctx)

			if tt.wantErr {
				if err == nil {
					t.Fatal("CanActivate() expected error, got nil")
				}
				// Verify it's a web access error with correct status.
				type statusCoder interface{ StatusCode() int }
				if sc, ok := err.(statusCoder); ok {
					if sc.StatusCode() != tt.wantStatus {
						t.Fatalf("StatusCode() = %d, want %d", sc.StatusCode(), tt.wantStatus)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("CanActivate() unexpected error = %v", err)
			}

			if tt.wantClaims {
				claims, ok := ClaimsFromContext(ctx)
				if !ok || claims == nil {
					t.Fatal("expected claims in context, got none")
				}
				if got, _ := claims["sub"].(string); got != "user-1" {
					t.Fatalf("sub = %q, want %q", got, "user-1")
				}
			}
		})
	}
}

func TestJWTGuard_ExpiredToken_Returns401(t *testing.T) {
	mapClaims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Second).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	token, _ := jwtToken.SignedString([]byte(testSecret))

	guard, _ := newTestGuard()
	ctx := newMockCtx("Bearer " + token)
	err := guard.CanActivate(ctx)
	if err == nil {
		t.Fatal("expected error for expired token")
	}

	type statusCoder interface{ StatusCode() int }
	if sc, ok := err.(statusCoder); ok {
		if sc.StatusCode() != http.StatusUnauthorized {
			t.Fatalf("StatusCode() = %d, want 401", sc.StatusCode())
		}
	}
}

// ─── ClaimsFromContext ────────────────────────────────────────────────────────

func TestClaimsFromContext_NoClaims(t *testing.T) {
	ctx := newMockCtx("")
	claims, ok := ClaimsFromContext(ctx)
	if ok || claims != nil {
		t.Fatal("expected (nil, false) when no claims are set")
	}
}

func TestClaimsFromContext_WithClaims(t *testing.T) {
	ctx := newMockCtx("")
	expected := map[string]any{"sub": "user-1"}
	ctx.Locals(claimsKey, expected)

	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("expected claims to be found")
	}
	if got, _ := claims["sub"].(string); got != "user-1" {
		t.Fatalf("sub = %q, want %q", got, "user-1")
	}
}

func TestClaimsFromContext_InvalidType(t *testing.T) {
	ctx := newMockCtx("")
	ctx.Locals(claimsKey, "not-a-map")

	_, ok := ClaimsFromContext(ctx)
	if ok {
		t.Fatal("expected false when locals value is not map[string]any")
	}
}

// ─── errors sentinel coverage ─────────────────────────────────────────────────

func TestErrors_Sentinels(t *testing.T) {
	if !errors.Is(ErrUnauthorized, ErrUnauthorized) {
		t.Error("ErrUnauthorized sentinel broken")
	}
	if !errors.Is(ErrForbidden, ErrForbidden) {
		t.Error("ErrForbidden sentinel broken")
	}
	if !errors.Is(ErrTokenExpired, ErrTokenExpired) {
		t.Error("ErrTokenExpired sentinel broken")
	}
	if !errors.Is(ErrTokenInvalid, ErrTokenInvalid) {
		t.Error("ErrTokenInvalid sentinel broken")
	}
}
