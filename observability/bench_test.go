package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enokdev/helix/web"
)

func BenchmarkActuatorHealthRoute(b *testing.B) {
	server := web.NewServer()
	checker, err := NewCompositeHealthChecker()
	if err != nil {
		b.Fatalf("NewCompositeHealthChecker() error = %v", err)
	}
	if err := RegisterActuatorRoutes(server, checker, staticInfoProvider{}); err != nil {
		b.Fatalf("RegisterActuatorRoutes() error = %v", err)
	}
	b.Cleanup(func() {
		_ = server.Stop(context.Background())
	})

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := server.ServeHTTP(req.Clone(context.Background()))
		if err != nil {
			b.Fatalf("ServeHTTP() error = %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		_ = resp.Body.Close()
	}
}

type staticInfoProvider struct{}

func (staticInfoProvider) Info(context.Context) InfoResponse {
	return InfoResponse{Version: "bench", Profiles: []string{}, Build: map[string]string{}}
}
