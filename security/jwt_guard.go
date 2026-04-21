package security

import (
	"strings"

	"github.com/enokdev/helix/web"
)

const claimsKey = "jwt_claims"

// JWTGuard validates the Authorization: Bearer header and stores parsed claims
// in the request context under the "jwt_claims" key.
type JWTGuard struct {
	svc JWTServicer
}

// NewJWTGuard creates a JWTGuard backed by the given JWTServicer.
func NewJWTGuard(svc JWTServicer) *JWTGuard {
	return &JWTGuard{svc: svc}
}

// CanActivate implements web.Guard. It rejects the request with 401 when no
// valid Bearer token is present, and stores the claims for use in handlers.
func (g *JWTGuard) CanActivate(ctx web.Context) error {
	header := ctx.Header("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return web.Unauthorized("missing or malformed Authorization header")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return web.Unauthorized("missing or malformed Authorization header")
	}

	claims, err := g.svc.Validate(token)
	if err != nil {
		return web.Unauthorized("invalid or expired token")
	}

	ctx.Locals(claimsKey, claims)
	return nil
}
