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
