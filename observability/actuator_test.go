package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/web"
)

func TestRegisterActuatorRoutesServesHealthAndInfo(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	checker, err := NewCompositeHealthChecker(
		&testHealthIndicator{name: "db", health: ComponentHealth{Status: StatusDown, Error: "connection refused"}},
	)
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	loader := config.NewLoader(config.WithAllowMissingConfig(), config.WithProfiles("test"))
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	info := NewInfoProvider(loader, WithVersion("1.0.0"), WithBuildInfo(map[string]string{"commit": "abc123"}))

	if err := RegisterActuatorRoutes(server, checker, info); err != nil {
		t.Fatalf("RegisterActuatorRoutes() error = %v", err)
	}

	healthResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/health", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(health) error = %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("health StatusCode = %d, want %d", healthResp.StatusCode, http.StatusServiceUnavailable)
	}
	var healthBody HealthResponse
	if err := json.NewDecoder(healthResp.Body).Decode(&healthBody); err != nil {
		t.Fatalf("decode health body: %v", err)
	}
	if healthBody.Status != StatusDown || healthBody.Components["db"].Error != "connection refused" {
		t.Fatalf("health body = %#v, want down db", healthBody)
	}

	infoResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/info", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(info) error = %v", err)
	}
	defer infoResp.Body.Close()
	if infoResp.StatusCode != http.StatusOK {
		t.Fatalf("info StatusCode = %d, want %d", infoResp.StatusCode, http.StatusOK)
	}
	var infoBody InfoResponse
	if err := json.NewDecoder(infoResp.Body).Decode(&infoBody); err != nil {
		t.Fatalf("decode info body: %v", err)
	}
	if infoBody.Version != "1.0.0" || len(infoBody.Profiles) != 1 || infoBody.Profiles[0] != "test" || infoBody.Build["commit"] != "abc123" {
		t.Fatalf("info body = %#v, want version/profile/build", infoBody)
	}
}

func TestRegisterActuatorRoutesServesExactUpHealthWithoutComponents(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	checker, err := NewCompositeHealthChecker()
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	if err := RegisterActuatorRoutes(server, checker, NewInfoProvider(nil)); err != nil {
		t.Fatalf("RegisterActuatorRoutes() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/health", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 1 || body["status"] != string(StatusUp) {
		t.Fatalf("body = %#v, want only status UP", body)
	}
}

func TestRegisterActuatorRoutesPropagatesRequestContextToHealth(t *testing.T) {
	t.Parallel()

	server := &captureHTTPServer{}
	indicator := &blockingHealthIndicator{name: "io", ready: make(chan struct{})}
	checker, err := NewCompositeHealthChecker(indicator)
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	if err := RegisterActuatorRoutes(server, checker, NewInfoProvider(nil)); err != nil {
		t.Fatalf("RegisterActuatorRoutes() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reqCtx := &actuatorTestContext{ctx: ctx}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.routes[http.MethodGet+" "+healthPath](reqCtx)
	}()

	// Wait until the indicator is blocking inside Health(), then cancel.
	<-indicator.ready
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("health handler did not return after context cancellation")
	}

	if !errors.Is(indicator.ctxErr, context.Canceled) {
		t.Fatalf("HealthIndicator ctx.Err() = %v, want context.Canceled", indicator.ctxErr)
	}
}

func TestRegisterActuatorRoutesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	checker, err := NewCompositeHealthChecker()
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	info := NewInfoProvider(nil)
	tests := []struct {
		name    string
		server  web.HTTPServer
		checker HealthChecker
		info    InfoProvider
	}{
		{name: "nil server", server: nil, checker: checker, info: info},
		{name: "nil checker", server: web.NewServer(), checker: nil, info: info},
		{name: "nil info", server: web.NewServer(), checker: checker, info: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := RegisterActuatorRoutes(tt.server, tt.checker, tt.info)
			if !errors.Is(err, ErrInvalidActuator) {
				t.Fatalf("RegisterActuatorRoutes() error = %v, want ErrInvalidActuator", err)
			}
		})
	}
}

func TestRegisterActuatorRoutesWrapsRouteRegistrationError(t *testing.T) {
	t.Parallel()

	checker, err := NewCompositeHealthChecker()
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	wantErr := errors.New("boom")
	err = RegisterActuatorRoutes(failingHTTPServer{err: wantErr}, checker, NewInfoProvider(nil))
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterActuatorRoutes() error = %v, want wrapped route error", err)
	}
}

func TestActuatorHealthP99LatencyBelowFiveMilliseconds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency guard in short mode")
	}
	t.Parallel()

	server := web.NewServer()
	checker, err := NewCompositeHealthChecker()
	if err != nil {
		t.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	if err := RegisterActuatorRoutes(server, checker, NewInfoProvider(nil)); err != nil {
		t.Fatalf("RegisterActuatorRoutes() error = %v", err)
	}

	const iterations = 200
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/health", nil))
		if err != nil {
			t.Fatalf("ServeHTTP() error = %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		resp.Body.Close()
		durations = append(durations, time.Since(start))
	}
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	p99 := durations[(iterations*99+99)/100-1]
	if p99 >= 5*time.Millisecond {
		t.Fatalf("p99 latency = %s, want < 5ms", p99)
	}
}

type failingHTTPServer struct {
	err error
}

func (s failingHTTPServer) Start(string) error {
	return nil
}

func (s failingHTTPServer) Stop(context.Context) error {
	return nil
}

func (s failingHTTPServer) RegisterRoute(string, string, web.HandlerFunc) error {
	return s.err
}

func (s failingHTTPServer) IsGeneratedOnly() bool {
	return false
}

func (s failingHTTPServer) ServeHTTP(*http.Request) (*http.Response, error) {
	return nil, nil
}

type captureHTTPServer struct {
	routes map[string]web.HandlerFunc
}

func (s *captureHTTPServer) Start(string) error {
	return nil
}

func (s *captureHTTPServer) Stop(context.Context) error {
	return nil
}

func (s *captureHTTPServer) RegisterRoute(method, path string, handler web.HandlerFunc) error {
	if s.routes == nil {
		s.routes = make(map[string]web.HandlerFunc)
	}
	s.routes[method+" "+path] = handler
	return nil
}

func (s *captureHTTPServer) IsGeneratedOnly() bool {
	return false
}

func (s *captureHTTPServer) ServeHTTP(*http.Request) (*http.Response, error) {
	return nil, nil
}

type actuatorTestContext struct {
	ctx context.Context
}

func (c *actuatorTestContext) Method() string              { return http.MethodGet }
func (c *actuatorTestContext) Path() string                { return healthPath }
func (c *actuatorTestContext) OriginalURL() string         { return healthPath }
func (c *actuatorTestContext) Param(string) string         { return "" }
func (c *actuatorTestContext) Query(string) string         { return "" }
func (c *actuatorTestContext) Header(string) string        { return "" }
func (c *actuatorTestContext) IP() string                  { return "" }
func (c *actuatorTestContext) Body() []byte                { return nil }
func (c *actuatorTestContext) Status(int)                  {}
func (c *actuatorTestContext) SetHeader(string, string)    {}
func (c *actuatorTestContext) AppendHeader(string, string) {}
func (c *actuatorTestContext) Send([]byte) error           { return nil }
func (c *actuatorTestContext) JSON(any) error              { return nil }
func (c *actuatorTestContext) Context() context.Context    { return c.ctx }
func (c *actuatorTestContext) Locals(string, ...any) any   { return nil }

// blockingHealthIndicator blocks inside Health() until the context is
// cancelled, allowing tests to verify that the request context is correctly
// propagated and that cancellation is observable from within the indicator.
type blockingHealthIndicator struct {
	name   string
	ready  chan struct{} // closed once Health() is entered
	ctxErr error
}

func (i *blockingHealthIndicator) Name() string { return i.name }

func (i *blockingHealthIndicator) Health(ctx context.Context) ComponentHealth {
	close(i.ready)   // signal: now blocking
	<-ctx.Done()     // wait for cancellation
	i.ctxErr = ctx.Err()
	return ComponentHealth{Status: StatusUp}
}
