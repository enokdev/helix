package observability

import (
	"context"
	"reflect"
	"testing"

	"github.com/enokdev/helix/config"
)

func TestInfoProviderDefaults(t *testing.T) {
	t.Parallel()

	provider := NewInfoProvider(nil)

	got := provider.Info(context.Background())
	want := InfoResponse{
		Version:  "dev",
		Profiles: []string{},
		Build:    map[string]string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Info() = %#v, want %#v", got, want)
	}
}

func TestInfoProviderUsesLoaderProfilesAndOptions(t *testing.T) {
	t.Parallel()

	loader := config.NewLoader(
		config.WithAllowMissingConfig(),
		config.WithProfiles("test", "local"),
	)
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	build := map[string]string{"commit": "abc123"}
	provider := NewInfoProvider(loader, WithVersion("1.2.3"), WithBuildInfo(build))
	build["commit"] = "mutated"

	got := provider.Info(context.Background())
	want := InfoResponse{
		Version:  "1.2.3",
		Profiles: []string{"test", "local"},
		Build:    map[string]string{"commit": "abc123"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Info() = %#v, want %#v", got, want)
	}

	got.Build["commit"] = "changed"
	again := provider.Info(context.Background())
	if again.Build["commit"] != "abc123" {
		t.Fatalf("Info() build map was mutated, got %q", again.Build["commit"])
	}
}
