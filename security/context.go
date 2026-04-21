package security

import "github.com/enokdev/helix/web"

// ClaimsFromContext extracts the JWT claims stored by JWTGuard from the
// request context. Returns (nil, false) when no claims are present.
func ClaimsFromContext(ctx web.Context) (map[string]any, bool) {
	v := ctx.Locals(claimsKey)
	if v == nil {
		return nil, false
	}
	claims, ok := v.(map[string]any)
	return claims, ok
}
