package security

import (
	"testing"

	"github.com/enokdev/helix/core"
	helixsecurity "github.com/enokdev/helix/security"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Load(any) error { return nil }
func (f fakeConfig) Lookup(key string) (any, bool) {
	v, ok := f.values[key]
	return v, ok
}
func (f fakeConfig) ConfigFileUsed() string      { return "" }
func (f fakeConfig) AllSettings() map[string]any { return f.values }
func (f fakeConfig) ActiveProfiles() []string    { return nil }

func newTestContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

// ─── Condition tests ──────────────────────────────────────────────────────────

func TestCondition(t *testing.T) {
	tests := []struct {
		name string
		cfg  fakeConfig
		want bool
	}{
		{
			name: "enabled: true",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: true}},
			want: true,
		},
		{
			name: "enabled: false",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: false}},
			want: false,
		},
		{
			name: "enabled: false overrides security.* key",
			cfg: fakeConfig{values: map[string]any{
				secEnabledKey: false,
				"security":    map[string]any{"jwt": map[string]any{"secret": "x"}},
			}},
			want: false,
		},
		{
			name: "auto-detect via security.* key",
			cfg:  fakeConfig{values: map[string]any{"security": map[string]any{"jwt": map[string]any{}}}},
			want: true,
		},
		{
			name: "no relevant keys",
			cfg:  fakeConfig{values: map[string]any{}},
			want: false,
		},
		{
			name: "enabled: int 0 → false",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: int(0)}},
			want: false,
		},
		{
			name: "enabled: int 1 → true",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: int(1)}},
			want: true,
		},
		{
			name: "enabled: float64 1 → true (YAML numeric)",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: float64(1)}},
			want: true,
		},
		{
			name: "enabled: float64 0 → false (YAML numeric)",
			cfg:  fakeConfig{values: map[string]any{secEnabledKey: float64(0)}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.cfg)
			if got := s.Condition(); got != tt.want {
				t.Fatalf("Condition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionNilConfig(t *testing.T) {
	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true with nil cfg, want false")
	}
}

// ─── Configure tests ──────────────────────────────────────────────────────────

func TestConfigureNilContainerIsNoop(_ *testing.T) {
	New(nil).Configure(nil)
}

func TestConfigureRegistersJWTService(t *testing.T) {
	container := newTestContainer()

	cfg := fakeConfig{values: map[string]any{
		jwtSecretKey: "my-secret",
		jwtExpiryKey: "1h",
	}}
	New(cfg).Configure(container)

	var svc *helixsecurity.JWTService
	if err := container.Resolve(&svc); err != nil {
		t.Fatalf("Resolve(*JWTService) error = %v", err)
	}
	if svc == nil {
		t.Fatal("JWTService is nil after Configure")
	}
}

func TestConfigureRegistersJWTService_DefaultsWithNilConfig(t *testing.T) {
	container := newTestContainer()

	// Empty secret (nil config) means no JWTService is registered — Configure must not panic.
	New(nil).Configure(container)

	var svc *helixsecurity.JWTService
	if err := container.Resolve(&svc); err == nil {
		t.Fatal("expected Resolve(*JWTService) to fail when secret is empty")
	}
}

func TestConfigureRegistersJWTService_InvalidExpiry_UsesDefault(t *testing.T) {
	container := newTestContainer()

	cfg := fakeConfig{values: map[string]any{
		jwtSecretKey: "my-secret",
		jwtExpiryKey: "not-a-duration",
	}}
	New(cfg).Configure(container)

	var svc *helixsecurity.JWTService
	if err := container.Resolve(&svc); err != nil {
		t.Fatalf("Resolve(*JWTService) error = %v", err)
	}
	if svc == nil {
		t.Fatal("JWTService is nil after Configure with invalid expiry")
	}
}
