# Helix Secured API Example

A complete example demonstrating JWT-based authentication and role-based access control (RBAC) in a Helix application.

## Overview

This example shows how to:
- Implement JWT token generation and validation
- Protect endpoints with authentication requirements
- Enforce role-based access control using Helix guards
- Configure global security rules with `helix.SecurityConfigurer`

## Demo Accounts

The example provides two pre-configured demo accounts:

| Username | Password | Roles          |
|----------|----------|----------------|
| `user`   | password | `user`         |
| `admin`  | password | `admin`, `user`|

**Note:** These accounts are demo fixtures only. In production, retrieve credentials from a secure database or identity provider and use strong password hashing like bcrypt.

## Running the Example

### From the Repository Root

```bash
go run ./examples/secured-api
```

The server starts on port `8081` (configurable via `examples/secured-api/config/application.yaml`).

### Running Tests

```bash
go test ./examples/secured-api
go test ./...
```

## API Endpoints

### Authentication

#### Login
```bash
POST /auth/login
Content-Type: application/json

{
  "username": "user",
  "password": "password"
}
```

**Success Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "username": "user",
  "role": "user"
}
```

**Error Response (401 Unauthorized):**
```json
{
  "error": {
    "type": "Unauthorized",
    "code": "UNAUTHORIZED",
    "message": "no valid token provided",
    "field": ""
  }
}
```

### Protected Endpoints

All protected endpoints require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

#### User Profile (Authenticated)
```bash
GET /api/profile
Authorization: Bearer <token>
```

**Success Response (200 OK):**
```json
{
  "username": "user",
  "role": "user"
}
```

**Error (401 Unauthorized):**
Sent when no token, malformed token, or expired token is provided.

#### Admin Users List (Admin Only)
```bash
GET /admin/users
Authorization: Bearer <token>
```

**Success Response (200 OK) - Admin Only:**
```json
{
  "users": [
    {
      "username": "user",
      "role": "user"
    },
    {
      "username": "admin",
      "role": "admin"
    }
  ]
}
```

**Error (401 Unauthorized):**
Sent when no valid token is provided.

**Error (403 Forbidden):**
Sent when the authenticated user does not have the `admin` role.

## Complete Authentication Flow

### 1. Login as User

```bash
$ TOKEN=$(curl -s -X POST http://localhost:8081/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"user","password":"password"}' | jq -r '.token')
```

### 2. Access User Profile

```bash
$ curl -i http://localhost:8081/api/profile \
  -H "Authorization: Bearer $TOKEN"
```

Expected: `200 OK` with profile data.

### 3. Attempt Admin Access with User Token

```bash
$ curl -i http://localhost:8081/admin/users \
  -H "Authorization: Bearer $TOKEN"
```

Expected: `403 Forbidden` (insufficient permissions).

### 4. Login as Admin and Access Admin Endpoint

```bash
$ ADMIN_TOKEN=$(curl -s -X POST http://localhost:8081/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"password"}' | jq -r '.token')

$ curl -i http://localhost:8081/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Expected: `200 OK` with user list.

## Implementation Details

### Security Configuration

The `SecurityConfig` struct uses `helix.SecurityConfigurer` to define global rules:

```go
func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
  hs.Route("/auth/**").PermitAll().
    Route("/api/**").Authenticated().
    Route("/admin/**").Authenticated()
}
```

Rules are matched in definition order; the first matching pattern applies.

### JWT Service

The `security.JWTService` handles token generation and validation:
- Uses HMAC-SHA256 (HS256) for signing
- Validates token signature and expiration
- Default expiry: 24 hours (configurable)
- Secret must be provided via `config/application.yaml` or environment variable

### Role-Based Guards

The `//helix:guard role:admin` directive on the `AdminController.Users` method enforces role checking:
- Guards are registered before controllers
- Roles are extracted from JWT claims
- Return `403 Forbidden` if the user lacks the required role

## Configuration

Edit `config/application.yaml` to customize:

```yaml
server:
  port: 8081          # HTTP server port
app:
  name: helix-secured-api
security:
  jwt:
    secret: "dev-only-secured-api-secret-change-me"  # Use a strong secret in production
    expiry: "1h"                                       # Token expiry duration
```

**Production Recommendations:**
- Use a strong, randomly generated secret (e.g., 32+ bytes, base64-encoded)
- Provide the secret via environment variable or secrets manager, never commit to version control
- Use HTTPS to protect tokens in transit
- Implement token rotation and refresh mechanisms
- Store secrets securely and rotate them periodically

## Architecture

- **AuthService**: In-memory demo account store and token generation
- **AuthController**: Handles `/auth/login` endpoint
- **APIController**: Provides authenticated user endpoint `/api/profile`
- **AdminController**: Provides admin-only endpoint `/admin/users`
- **SecurityConfig**: Defines global authentication and authorization rules
- **appServer**: Wraps the HTTP server for Helix lifecycle management

## Testing

The example includes comprehensive tests covering:
- Configuration loading with defaults
- Successful login for user and admin accounts
- Invalid credential rejection
- Protected endpoint access without token (401)
- Protected endpoint access with valid token (200)
- RBAC enforcement with insufficient permissions (403)
- RBAC enforcement with sufficient permissions (200)
- Documentation and code structure validation

Run tests with:
```bash
go test -v ./examples/secured-api
```

## Helix APIs Used

- `helix.Service`, `helix.Controller`, `helix.Component` component markers
- `security.NewJWTService()` for token generation and validation
- `security.ClaimsFromContext()` to read JWT claims in handlers
- `security.HTTPSecurity` for global security configuration
- `security.NewRoleGuardFactory()` for role-based access control
- `web.RegisterGuardFactory()` to register guard factories
- `web.ApplyGlobalGuard()` to apply global security rules
- `web.Unauthorized()` and `web.Forbidden()` for standard error responses
- `//helix:guard role:admin` directive for declarative RBAC
- `config.NewLoader()` for YAML configuration with environment overrides

## References

- [Helix Security Documentation](../../docs/security-observability-scheduling.md)
- [JWT Guard and RBAC Guide](../../docs/security-observability-scheduling.md#authentication--authorization)
- [CRUD API Example](../crud-api/README.md)
