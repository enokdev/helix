package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/security"
	"github.com/enokdev/helix/web"
)

type appConfig struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
	Security struct {
		JWT struct {
			Secret string        `mapstructure:"secret"`
			Expiry time.Duration `mapstructure:"expiry"`
		} `mapstructure:"jwt"`
	} `mapstructure:"security"`
}

// authError is a custom error type for authentication errors.
type authError struct {
	status  int
	message string
}

func (e *authError) Error() string      { return e.message }
func (e *authError) StatusCode() int    { return e.status }
func (e *authError) ErrorType() string  { return "AuthenticationError" }
func (e *authError) ErrorCode() string  { return "AUTH_ERROR" }
func (e *authError) ErrorField() string { return "" }

func authenticationFailed() error {
	return &authError{status: 401, message: "invalid credentials"}
}

func internalError(msg string) error {
	return &authError{status: 500, message: msg}
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int64  `json:"expires_in"`
	Username  string `json:"username"`
	Role      string `json:"role"`
}

type AccountInfo struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UserList struct {
	Users []AccountInfo `json:"users"`
}

// DemoAccount represents an in-memory user account.
type DemoAccount struct {
	Username string
	Password string
	Roles    []string
}

// AuthService handles authentication logic.
type AuthService struct {
	helix.Service

	JWTSvc   *security.JWTService `inject:"true"`
	accounts map[string]DemoAccount
	mu       sync.Mutex
}

func NewAuthService() *AuthService {
	return &AuthService{
		accounts: map[string]DemoAccount{
			"user": {
				Username: "user",
				Password: "password",
				Roles:    []string{"user"},
			},
			"admin": {
				Username: "admin",
				Password: "password",
				Roles:    []string{"admin", "user"},
			},
		},
	}
}

func (s *AuthService) Authenticate(username, password string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, exists := s.accounts[username]
	if !exists || account.Password != password {
		return nil, authenticationFailed()
	}

	claims := map[string]any{
		"username": account.Username,
		"roles":    account.Roles,
	}
	return claims, nil
}

func (s *AuthService) GetAllAccounts() []AccountInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	var accounts []AccountInfo
	for _, account := range s.accounts {
		accounts = append(accounts, AccountInfo{
			Username: account.Username,
			Role:     account.Roles[0],
		})
	}
	return accounts
}

// AuthController handles authentication endpoints.
type AuthController struct {
	helix.Controller

	AuthSvc *AuthService `inject:"true"`
	JWTSvc  *security.JWTService `inject:"true"`
}

func NewAuthController() *AuthController {
	return &AuthController{}
}

//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context, req LoginRequest) (LoginResponse, error) {
	claims, err := c.AuthSvc.Authenticate(req.Username, req.Password)
	if err != nil {
		return LoginResponse{}, err
	}

	token, err := c.JWTSvc.Generate(claims)
	if err != nil {
		return LoginResponse{}, internalError("failed to generate token")
	}

	expiresIn := time.Now().Add(time.Hour).Unix() - time.Now().Unix()

	// Set custom status code for this action endpoint (201 is framework default for POST)
	ctx.Locals("_helix_custom_status", http.StatusOK)

	return LoginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: expiresIn,
		Username:  req.Username,
		Role:      claims["roles"].([]string)[0],
	}, nil
}

// APIController handles user API endpoints.
type APIController struct {
	helix.Controller

	AuthSvc *AuthService `inject:"true"`
}

func NewAPIController() *APIController {
	return &APIController{}
}

//helix:route GET /api/profile
func (c *APIController) Profile(ctx web.Context) (AccountInfo, error) {
	claims, ok := security.ClaimsFromContext(ctx)
	if !ok {
		return AccountInfo{}, web.Unauthorized("no claims found")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return AccountInfo{}, internalError("invalid username in token")
	}

	roles, ok := claims["roles"].([]any)
	if !ok || len(roles) == 0 {
		return AccountInfo{}, internalError("invalid roles in token")
	}

	role, _ := roles[0].(string)

	return AccountInfo{
		Username: username,
		Role:     role,
	}, nil
}

// AdminController handles admin-only endpoints.
type AdminController struct {
	helix.Controller

	AuthSvc *AuthService `inject:"true"`
}

func NewAdminController() *AdminController {
	return &AdminController{}
}

//helix:route GET /admin/users
//helix:guard role:admin
func (c *AdminController) Users(ctx web.Context) (UserList, error) {
	accounts := c.AuthSvc.GetAllAccounts()
	return UserList{Users: accounts}, nil
}

// SecurityConfig applies global security rules.
type SecurityConfig struct {
	helix.SecurityConfigurer

	JWTSvc *security.JWTService `inject:"true"`
}

func NewSecurityConfig() *SecurityConfig {
	return &SecurityConfig{}
}

func (sc *SecurityConfig) Configure(hs *security.HTTPSecurity) {
	hs.Route("/auth/**").PermitAll().
		Route("/api/**").Authenticated().
		Route("/admin/**").Authenticated()
}

// appServer wraps HTTPServer to participate in the Helix lifecycle.
type appServer struct {
	helix.Component

	server web.HTTPServer
	addr   string
}

func (s *appServer) OnStart() error {
	return s.server.Start(s.addr)
}

func (s *appServer) OnStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Stop(ctx)
}

func loadConfig() (appConfig, error) {
	loader := config.NewLoader(
		config.WithConfigPaths("examples/secured-api/config", "config"),
		config.WithDefaults(map[string]any{
			"server.port":          8081,
			"app.name":             "helix-secured-api",
			"security.jwt.secret":  "dev-only-secured-api-secret-change-me",
			"security.jwt.expiry":  1 * time.Hour,
		}),
	)

	var cfg appConfig
	if err := loader.Load(&cfg); err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}

func newServer(jwtSvc *security.JWTService) (web.HTTPServer, error) {
	container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))
	for _, component := range []any{
		jwtSvc,
		NewAuthService(),
		&AuthController{},
		&APIController{},
		&AdminController{},
		&SecurityConfig{},
	} {
		if err := container.Register(component); err != nil {
			return nil, err
		}
	}

	var authCtrl *AuthController
	var apiCtrl *APIController
	var adminCtrl *AdminController
	var securityCfg *SecurityConfig

	if err := container.Resolve(&authCtrl); err != nil {
		return nil, err
	}
	if err := container.Resolve(&apiCtrl); err != nil {
		return nil, err
	}
	if err := container.Resolve(&adminCtrl); err != nil {
		return nil, err
	}
	if err := container.Resolve(&securityCfg); err != nil {
		return nil, err
	}

	server := web.NewServer()

	// Register guard factory for role-based access control
	if err := web.RegisterGuardFactory(server, "role", security.NewRoleGuardFactory()); err != nil {
		return nil, err
	}

	// Register controllers
	if err := web.RegisterController(server, authCtrl); err != nil {
		return nil, err
	}
	if err := web.RegisterController(server, apiCtrl); err != nil {
		return nil, err
	}
	if err := web.RegisterController(server, adminCtrl); err != nil {
		return nil, err
	}

	// Apply global security configuration
	httpSecurity := security.NewHTTPSecurity(jwtSvc)
	securityCfg.Configure(httpSecurity)
	globalGuard := httpSecurity.Build()
	if err := web.ApplyGlobalGuard(server, globalGuard); err != nil {
		return nil, err
	}

	return server, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		log.Fatalf("invalid server port: %d", cfg.Server.Port)
	}

	jwtSvc, err := security.NewJWTService(cfg.Security.JWT.Secret, cfg.Security.JWT.Expiry)
	if err != nil {
		log.Fatal(err)
	}

	server, err := newServer(jwtSvc)
	if err != nil {
		log.Fatal(err)
	}

	if err := helix.Run(helix.App{
		Components: []any{&appServer{server: server, addr: ":" + strconv.Itoa(cfg.Server.Port)}},
	}); err != nil {
		log.Fatal(err)
	}
}
