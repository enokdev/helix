package security

import (
	"net/url"
	"strings"

	"github.com/enokdev/helix/web"
)

// Configurer is the interface that global security configuration components implement.
// Embed helix.SecurityConfigurer in your struct to auto-register it.
type Configurer interface {
	Configure(hs *HTTPSecurity)
}

type pathRule struct {
	pattern string
	guard   web.Guard
}

// HTTPSecurity accumulates security rules based on path patterns.
// Created by the framework during startup and passed to Configurer.Configure().
type HTTPSecurity struct {
	jwtSvc JWTServicer
	rules  []pathRule
}

// NewHTTPSecurity creates an HTTPSecurity builder.
func NewHTTPSecurity(svc JWTServicer) *HTTPSecurity {
	return &HTTPSecurity{
		jwtSvc: svc,
	}
}

// Route returns a RouteSecurityBuilder scoped to the given path pattern.
// Wildcards: ** (all segments) and * (one segment).
func (hs *HTTPSecurity) Route(pattern string) *RouteSecurityBuilder {
	return &RouteSecurityBuilder{
		hs:      hs,
		pattern: pattern,
	}
}

// Build constructs the global guard that applies rules in definition order.
func (hs *HTTPSecurity) Build() web.Guard {
	rules := make([]pathRule, len(hs.rules))
	copy(rules, hs.rules)
	return web.GuardFunc(func(ctx web.Context) error {
		path := ctx.Path()
		for _, rule := range rules {
			if matchesPattern(rule.pattern, path) {
				return rule.guard.CanActivate(ctx)
			}
		}
		return nil // Allow if no rule matches
	})
}

// RouteSecurityBuilder is an intermediate builder for a specific path pattern.
type RouteSecurityBuilder struct {
	hs      *HTTPSecurity
	pattern string
}

// PermitAll allows all requests for this pattern without authentication.
func (b *RouteSecurityBuilder) PermitAll() *HTTPSecurity {
	b.hs.rules = append(b.hs.rules, pathRule{
		pattern: b.pattern,
		guard: web.GuardFunc(func(_ web.Context) error {
			return nil
		}),
	})
	return b.hs
}

// Authenticated requires a valid JWT token.
func (b *RouteSecurityBuilder) Authenticated() *HTTPSecurity {
	var guard web.Guard
	if b.hs.jwtSvc == nil {
		guard = web.GuardFunc(func(_ web.Context) error {
			return web.Unauthorized("authentication required but no JWT service is configured")
		})
	} else {
		guard = NewJWTGuard(b.hs.jwtSvc)
	}
	b.hs.rules = append(b.hs.rules, pathRule{
		pattern: b.pattern,
		guard:   guard,
	})
	return b.hs
}

// HasRole requires the user to have at least one of the specified roles.
// If no roles are provided, all requests are denied with 401.
// For unauthenticated requests (no JWT claims), returns 401 instead of 403.
func (b *RouteSecurityBuilder) HasRole(roles ...string) *HTTPSecurity {
	var guard web.Guard
	if len(roles) == 0 {
		guard = web.GuardFunc(func(_ web.Context) error {
			return web.Unauthorized("security: HasRole requires at least one role")
		})
	} else {
		roleGuard := NewRoleGuard(roles...)
		guard = web.GuardFunc(func(ctx web.Context) error {
			if _, ok := ClaimsFromContext(ctx); !ok {
				return web.Unauthorized("authentication required")
			}
			return roleGuard.CanActivate(ctx)
		})
	}
	b.hs.rules = append(b.hs.rules, pathRule{
		pattern: b.pattern,
		guard:   guard,
	})
	return b.hs
}

// matchesPattern matches a path against a pattern supporting * and **.
func matchesPattern(pattern, path string) bool {
	pattern = normalizeSecurityPath(pattern)
	path = normalizeSecurityPath(path)
	if pattern == "/" {
		return path == "/" || path == ""
	}

	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")

	return matchSegments(patternSegments, pathSegments)
}

func normalizeSecurityPath(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func matchSegments(pattern, path []string) bool {
	pIdx, tIdx := 0, 0
	for pIdx < len(pattern) && tIdx < len(path) {
		p := pattern[pIdx]
		if p == "**" {
			// ** as the last segment matches everything remaining.
			if pIdx == len(pattern)-1 {
				return true
			}
			// ** in the middle: try every possible split point (including zero segments consumed).
			for i := tIdx; i <= len(path); i++ {
				if matchSegments(pattern[pIdx+1:], path[i:]) {
					return true
				}
			}
			return false
		} else if p == "*" {
			pIdx++
			tIdx++
		} else {
			if path[tIdx] != p {
				return false
			}
			pIdx++
			tIdx++
		}
	}

	// Any remaining pattern segments must all be **.
	for i := pIdx; i < len(pattern); i++ {
		if pattern[i] != "**" {
			return false
		}
	}

	return tIdx == len(path)
}
