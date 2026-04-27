package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTServicer is the contract for JWT operations.
type JWTServicer interface {
	Generate(claims map[string]any) (string, error)
	Validate(token string) (map[string]any, error)
	Refresh(token string) (string, error)
}

// JWTService signs and validates JWT tokens using HS256.
type JWTService struct {
	secret []byte
	expiry time.Duration
}

// NewJWTService creates a JWTService with the given secret and token expiry.
// A zero expiry defaults to 24 hours. An empty secret returns an error.
func NewJWTService(secret string, expiry time.Duration) (*JWTService, error) {
	if secret == "" {
		return nil, fmt.Errorf("security: new jwt service: %w", ErrTokenInvalid)
	}
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}
	return &JWTService{secret: []byte(secret), expiry: expiry}, nil
}

// Generate creates a signed JWT embedding the provided claims plus an "exp" field.
func (s *JWTService) Generate(claims map[string]any) (string, error) {
	mapClaims := make(jwt.MapClaims, len(claims)+1)
	for k, v := range claims {
		mapClaims[k] = v
	}
	mapClaims["exp"] = time.Now().Add(s.expiry).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("security: generate token: %w", err)
	}
	return signed, nil
}

// Validate parses and verifies the token, returning its claims on success.
// Returns ErrTokenExpired or ErrTokenInvalid on failure.
func (s *JWTService) Validate(tokenStr string) (map[string]any, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("security: unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("security: validate token: %w", ErrTokenExpired)
		}
		return nil, fmt.Errorf("security: validate token: %w", ErrTokenInvalid)
	}

	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("security: validate token: %w", ErrTokenInvalid)
	}
	result := make(map[string]any, len(mapClaims))
	for k, v := range mapClaims {
		result[k] = v
	}
	return result, nil
}

// Refresh validates the token and returns a new one with a reset expiry.
// Returns ErrTokenExpired if the token is already expired.
func (s *JWTService) Refresh(tokenStr string) (string, error) {
	claims, err := s.Validate(tokenStr)
	if err != nil {
		return "", fmt.Errorf("security: refresh token: %w", err)
	}
	delete(claims, "exp")
	return s.Generate(claims)
}
