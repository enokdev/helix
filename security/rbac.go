package security

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/enokdev/helix/web"
)

// Compile-time assertion: RoleGuard must satisfy web.Guard.
var _ web.Guard = (*RoleGuard)(nil)

// RoleGuard implements web.Guard to enforce role-based access control (RBAC).
// It reads the "roles" claim from JWT claims previously stored in the request
// context by JWTGuard, and allows the request only when the user possesses
// at least one of the allowed roles.
//
// Usage with the //helix:guard directive:
//
//	//helix:guard role:admin
//	//helix:guard role:admin,moderator
type RoleGuard struct {
	allowedRoles []string
}

// NewRoleGuard creates a RoleGuard that permits access when the authenticated
// user has at least one of the given roles. It panics if called with no roles.
func NewRoleGuard(allowedRoles ...string) *RoleGuard {
	if len(allowedRoles) == 0 {
		panic("security: NewRoleGuard requires at least one allowed role")
	}
	cp := make([]string, len(allowedRoles))
	copy(cp, allowedRoles)
	return &RoleGuard{allowedRoles: cp}
}

// NewRoleGuardFactory returns a web.GuardFactory that parses a comma-separated
// roles argument (as produced by //helix:guard role:admin,moderator) and creates
// a RoleGuard from the result.
func NewRoleGuardFactory() web.GuardFactory {
	return func(argument string) (web.Guard, error) {
		parts := strings.Split(argument, ",")
		roles := make([]string, 0, len(parts))
		for _, r := range parts {
			r = strings.TrimSpace(r)
			if r != "" {
				roles = append(roles, r)
			}
		}
		if len(roles) == 0 {
			return nil, fmt.Errorf("security: role guard factory: %q contains no valid role", argument)
		}
		return NewRoleGuard(roles...), nil
	}
}

// CanActivate implements web.Guard. It returns a 403 Forbidden error when:
//   - no JWT claims are present in the request context (unauthenticated)
//   - the "roles" claim is missing or is not a []any or []string slice
//   - none of the user's roles match any of the allowed roles
func (g *RoleGuard) CanActivate(ctx web.Context) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return newForbiddenErr()
	}

	rawRoles, exists := claims["roles"]
	if !exists {
		return newForbiddenErr()
	}

	var roles []string
	switch v := rawRoles.(type) {
	case []any:
		for _, raw := range v {
			if s, ok := raw.(string); ok {
				roles = append(roles, s)
			}
		}
	case []string:
		roles = v
	default:
		return newForbiddenErr()
	}

	for _, role := range roles {
		for _, allowed := range g.allowedRoles {
			if role == allowed {
				return nil
			}
		}
	}

	return newForbiddenErr()
}

// rbacError is a 403 Forbidden error that wraps ErrForbidden so that
// errors.Is(err, security.ErrForbidden) returns true.
type rbacError struct {
	msg string
}

func newForbiddenErr() error {
	return &rbacError{msg: "Insufficient permissions"}
}

func (e *rbacError) Error() string      { return e.msg }
func (e *rbacError) StatusCode() int    { return http.StatusForbidden }
func (e *rbacError) ErrorType() string  { return "Forbidden" }
func (e *rbacError) ErrorCode() string  { return "FORBIDDEN" }
func (e *rbacError) ErrorField() string { return "" }
func (e *rbacError) Unwrap() error      { return ErrForbidden }
