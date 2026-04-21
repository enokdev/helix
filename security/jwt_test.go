package security

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "super-secret-key-for-tests"

func newTestService() *JWTService {
	svc, err := NewJWTService(testSecret, time.Hour)
	if err != nil {
		panic(err)
	}
	return svc
}

// ─── Generate ────────────────────────────────────────────────────────────────

func TestJWTService_Generate(t *testing.T) {
	tests := []struct {
		name    string
		claims  map[string]any
		wantErr bool
	}{
		{
			name:   "valid claims",
			claims: map[string]any{"sub": "user-1", "role": "admin"},
		},
		{
			name:   "empty claims",
			claims: map[string]any{},
		},
		{
			name:   "custom exp overridden",
			claims: map[string]any{"sub": "user-2", "exp": int64(1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService()
			token, err := svc.Generate(tt.claims)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Generate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && token == "" {
				t.Fatal("Generate() returned empty token")
			}
		})
	}
}

func TestJWTService_Generate_ExpiryIsSet(t *testing.T) {
	svc := newTestService()
	token, err := svc.Generate(map[string]any{"sub": "user-1"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Validate and confirm the exp claim is present and in the future.
	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	exp, ok := claims["exp"]
	if !ok {
		t.Fatal("exp claim missing from generated token")
	}
	expFloat, ok := exp.(float64)
	if !ok {
		t.Fatalf("exp claim has unexpected type %T", exp)
	}
	if int64(expFloat) <= time.Now().Unix() {
		t.Fatal("exp is not in the future")
	}
}

func TestJWTService_Generate_CustomExpiry(t *testing.T) {
	svc, err := NewJWTService(testSecret, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	before := time.Now().Add(4 * time.Minute).Unix()

	token, err := svc.Generate(map[string]any{"sub": "x"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	claims, _ := svc.Validate(token)
	expFloat, _ := claims["exp"].(float64)
	if int64(expFloat) < before {
		t.Fatal("expiry is shorter than expected")
	}
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestJWTService_Validate(t *testing.T) {
	svc := newTestService()
	validToken, _ := svc.Generate(map[string]any{"sub": "user-1", "role": "admin"})

	tests := []struct {
		name      string
		token     string
		wantErr   error
		wantSub   string
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantSub: "user-1",
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: ErrTokenInvalid,
		},
		{
			name:    "malformed token",
			token:   "not.a.jwt",
			wantErr: ErrTokenInvalid,
		},
		{
			name:    "wrong signature",
			token:   validToken + "tampered",
			wantErr: ErrTokenInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.Validate(tt.token)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error = %v", err)
			}
			if got, _ := claims["sub"].(string); got != tt.wantSub {
				t.Fatalf("sub claim = %q, want %q", got, tt.wantSub)
			}
		})
	}
}

func TestJWTService_Validate_ExpiredToken(t *testing.T) {
	// Build an expired token directly via the JWT library.
	mapClaims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	svc := newTestService()
	_, err = svc.Validate(signed)
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("Validate() error = %v, want ErrTokenExpired", err)
	}
}

func TestJWTService_Validate_WrongSecret(t *testing.T) {
	// A token signed by a different secret should fail HMAC verification.
	otherSvc, err := NewJWTService("different-secret", time.Hour)
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	token, _ := otherSvc.Generate(map[string]any{"sub": "attacker"})

	svc := newTestService()
	_, err = svc.Validate(token)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Validate() error = %v, want ErrTokenInvalid", err)
	}
}

func TestJWTService_Validate_WrongAlgorithm(t *testing.T) {
	// A token signed with the "none" algorithm must be rejected by the HMAC guard.
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": "attacker"})
	signed, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	svc := newTestService()
	_, err = svc.Validate(signed)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Validate() error = %v, want ErrTokenInvalid", err)
	}
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

func TestJWTService_Refresh(t *testing.T) {
	svc := newTestService()
	original, _ := svc.Generate(map[string]any{"sub": "user-1", "role": "admin"})

	refreshed, err := svc.Refresh(original)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if refreshed == "" {
		t.Fatal("Refresh() returned empty token")
	}

	// The refreshed token must be valid and carry the same non-exp claims.
	claims, err := svc.Validate(refreshed)
	if err != nil {
		t.Fatalf("Validate(refreshed) error = %v", err)
	}
	if got, _ := claims["sub"].(string); got != "user-1" {
		t.Fatalf("sub claim = %q, want %q", got, "user-1")
	}
	if got, _ := claims["role"].(string); got != "admin" {
		t.Fatalf("role claim = %q, want %q", got, "admin")
	}
}

func TestJWTService_Refresh_ExpiredToken(t *testing.T) {
	mapClaims := jwt.MapClaims{
		"sub": "user-1",
		"exp": time.Now().Add(-time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	svc := newTestService()
	_, err = svc.Refresh(signed)
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("Refresh() error = %v, want ErrTokenExpired", err)
	}
}

func TestJWTService_Refresh_InvalidToken(t *testing.T) {
	svc := newTestService()
	_, err := svc.Refresh("not-a-token")
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Refresh() error = %v, want ErrTokenInvalid", err)
	}
}

// ─── Default expiry ───────────────────────────────────────────────────────────

func TestNewJWTService_ZeroExpiryDefaultsTo24h(t *testing.T) {
	svc, err := NewJWTService(testSecret, 0)
	if err != nil {
		t.Fatalf("NewJWTService() error = %v", err)
	}
	if svc.expiry != 24*time.Hour {
		t.Fatalf("expiry = %v, want 24h", svc.expiry)
	}
}

func TestNewJWTService_EmptySecretReturnsError(t *testing.T) {
	_, err := NewJWTService("", time.Hour)
	if err == nil {
		t.Fatal("NewJWTService() expected error for empty secret, got nil")
	}
}
