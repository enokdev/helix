package observability_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enokdev/helix/observability"
	"github.com/enokdev/helix/web"
	"github.com/prometheus/client_golang/prometheus"
)

// ---------- Registry ----------

func TestNewRegistry_RetourneRegistreNonNil(t *testing.T) {
	t.Parallel()

	reg, err := observability.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if reg == nil {
		t.Fatal("NewRegistry() returned nil registry")
	}
}

func TestNewRegistry_RetourneRegistresIndependants(t *testing.T) {
	t.Parallel()

	r1, err := observability.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() #1 error = %v", err)
	}
	r2, err := observability.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() #2 error = %v", err)
	}
	if r1 == r2 {
		t.Fatal("NewRegistry() should return independent instances")
	}
}

func TestRegistry_RetourneSingleton(t *testing.T) {
	r1 := observability.Registry()
	r2 := observability.Registry()
	if r1 != r2 {
		t.Fatal("Registry() should always return the same instance")
	}
	if r1 == nil {
		t.Fatal("Registry() returned nil")
	}
}

// ---------- HTTPMetricsObserver ----------

func TestNewHTTPMetricsObserver_NilRegistry(t *testing.T) {
	t.Parallel()

	_, err := observability.NewHTTPMetricsObserver(nil)
	if err == nil {
		t.Fatal("NewHTTPMetricsObserver(nil) should return an error")
	}
}

func TestNewHTTPMetricsObserver_RegistreValide(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	obs, err := observability.NewHTTPMetricsObserver(reg)
	if err != nil {
		t.Fatalf("NewHTTPMetricsObserver() error = %v", err)
	}
	if obs == nil {
		t.Fatal("NewHTTPMetricsObserver() returned nil observer")
	}
}

func TestHTTPMetricsObserver_CollecteMetriques(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	obs, err := observability.NewHTTPMetricsObserver(reg)
	if err != nil {
		t.Fatalf("NewHTTPMetricsObserver() error = %v", err)
	}

	server := web.NewServer(web.WithRouteObserver(obs))
	if err := server.RegisterRoute(http.MethodGet, "/users/:id", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"id": ctx.Param("id")})
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	for _, id := range []string{"1", "2"} {
		resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
		if err != nil {
			t.Fatalf("ServeHTTP(/users/%s) error = %v", id, err)
		}
		resp.Body.Close()
	}

	metricServer := web.NewServer()
	if err := observability.RegisterMetricsRoute(metricServer, reg); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}
	resp, err := metricServer.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(/actuator/metrics) error = %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	if !strings.Contains(text, `route="/users/:id"`) {
		t.Errorf("expected route template label '/users/:id' in metrics, got:\n%s", text)
	}
	if !strings.Contains(text, "helix_http_requests_total") {
		t.Errorf("expected 'helix_http_requests_total' in metrics, got:\n%s", text)
	}
	if !strings.Contains(text, "helix_http_request_duration_seconds") {
		t.Errorf("expected 'helix_http_request_duration_seconds' in metrics, got:\n%s", text)
	}
}

func TestHTTPMetricsObserver_LabelRouteTemplateEtPasURLBrute(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	obs, _ := observability.NewHTTPMetricsObserver(reg)

	server := web.NewServer(web.WithRouteObserver(obs))
	if err := server.RegisterRoute(http.MethodGet, "/items/:id", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{})
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	for _, id := range []string{"abc", "xyz"} {
		resp, _ := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/items/"+id, nil))
		resp.Body.Close()
	}

	metricServer := web.NewServer()
	_ = observability.RegisterMetricsRoute(metricServer, reg)
	resp, _ := metricServer.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	if strings.Contains(text, `route="/items/abc"`) || strings.Contains(text, `route="/items/xyz"`) {
		t.Error("metrics must not expose raw URL values as route label")
	}
	if !strings.Contains(text, `route="/items/:id"`) {
		t.Errorf("expected route template '/items/:id' in metrics, got:\n%s", text)
	}
}

// ---------- RegisterMetricsRoute ----------

func TestRegisterMetricsRoute_ServerNil(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	err := observability.RegisterMetricsRoute(nil, reg)
	if err == nil {
		t.Fatal("RegisterMetricsRoute(nil, reg) should return an error")
	}
}

func TestRegisterMetricsRoute_RegistryNil(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	err := observability.RegisterMetricsRoute(server, nil)
	if err == nil {
		t.Fatal("RegisterMetricsRoute(server, nil) should return an error")
	}
}

func TestRegisterMetricsRoute_ExpositionTextePrometheus(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "helix_test_gauge",
		Help: "A gauge for exposition format test.",
	})
	_ = reg.Register(g)

	server := web.NewServer()
	if err := observability.RegisterMetricsRoute(server, reg); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") || !strings.Contains(ct, "version=0.0.4") {
		t.Errorf("Content-Type = %q, want text/plain; version=0.0.4; charset=utf-8", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if !strings.Contains(text, "# HELP") || !strings.Contains(text, "# TYPE") {
		t.Errorf("expected Prometheus text format with # HELP and # TYPE, got:\n%s", text)
	}
}

func TestRegisterMetricsRoute_CollectorCustomApparait(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "helix",
		Subsystem: "jobs",
		Name:      "queued",
		Help:      "Number of queued jobs.",
	})
	if err := reg.Register(gauge); err != nil {
		t.Fatalf("reg.Register() error = %v", err)
	}
	gauge.Set(7)

	server := web.NewServer()
	if err := observability.RegisterMetricsRoute(server, reg); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "helix_jobs_queued") {
		t.Errorf("custom collector not found in metrics output:\n%s", body)
	}
}

func TestRegisterMetricsRoute_GuardOptionnelRefuse(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	server := web.NewServer()

	blockGuard := web.GuardFunc(func(_ web.Context) error {
		return web.Forbidden("blocked")
	})

	if err := observability.RegisterMetricsRoute(server, reg, observability.WithMetricsGuard(blockGuard)); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("StatusCode = %d, want 403 (guard should block)", resp.StatusCode)
	}
}

func TestRegisterMetricsRoute_GuardParDefautPublic(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	server := web.NewServer()

	if err := observability.RegisterMetricsRoute(server, reg); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200 (default public endpoint)", resp.StatusCode)
	}
}

func TestRegisterMetricsRoute_GuardNilTypedRefuse(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	server := web.NewServer()
	var guard web.Guard // typed nil

	err := observability.RegisterMetricsRoute(server, reg, observability.WithMetricsGuard(guard))
	if err == nil {
		t.Fatal("RegisterMetricsRoute with typed-nil guard should return an error")
	}
}

func TestRegisterMetricsRoute_ErrorWrappee(t *testing.T) {
	t.Parallel()

	err := observability.RegisterMetricsRoute(nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "observability:") {
		t.Errorf("error should start with 'observability:', got %q", err.Error())
	}
}

func TestRegisterMetricsRoute_ErrorWrap_RegistryNil(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	err := observability.RegisterMetricsRoute(server, nil)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
	if !strings.HasPrefix(err.Error(), "observability:") {
		t.Errorf("error should start with 'observability:', got %q", err.Error())
	}
}

func TestRegisterMetricsRoute_GuardOptionnelUnauthorized(t *testing.T) {
	t.Parallel()

	reg, _ := observability.NewRegistry()
	server := web.NewServer()

	authGuard := web.GuardFunc(func(_ web.Context) error {
		return web.Unauthorized("not authenticated")
	})

	if err := observability.RegisterMetricsRoute(server, reg, observability.WithMetricsGuard(authGuard)); err != nil {
		t.Fatalf("RegisterMetricsRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want 401 (guard should reject unauthenticated)", resp.StatusCode)
	}
}
