package config

import (
	"reflect"
	"testing"
)

func TestParseProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "empty value",
			value: "",
			want:  nil,
		},
		{
			name:  "single profile",
			value: "prod",
			want:  []string{"prod"},
		},
		{
			name:  "multiple profiles",
			value: "dev,local",
			want:  []string{"dev", "local"},
		},
		{
			name:  "spaces and empty entries",
			value: " dev, ,local,, ",
			want:  []string{"dev", "local"},
		},
		{
			name:  "preserves case",
			value: "Prod,LOCAL",
			want:  []string{"Prod", "LOCAL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := profilesFromEnv(tt.value)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("profilesFromEnv(%q) = %#v, want %#v", tt.value, got, tt.want)
			}
		})
	}
}

func TestLoaderUsesProfilesFromEnv(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
  mode: base-mode
`)
	writeConfigFileInDir(t, configDir, "application-prod.yaml", `
server:
  port: 8081
app:
  mode: prod-mode
`)
	t.Setenv(activeProfilesEnv, "prod")
	t.Setenv("SERVER_PORT", "9090")

	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want ENV override 9090", cfg.Server.Port)
	}
	if cfg.App.Name != "base" {
		t.Fatalf("cfg.App.Name = %q, want inherited base", cfg.App.Name)
	}
	if cfg.App.Mode != "prod-mode" {
		t.Fatalf("cfg.App.Mode = %q, want profile override prod-mode", cfg.App.Mode)
	}
	if got, want := loader.ActiveProfiles(), []string{"prod"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ActiveProfiles() = %#v, want %#v", got, want)
	}
}

func TestLoaderWithoutActiveProfilesLoadsOnlyBaseConfig(t *testing.T) {
	t.Setenv(activeProfilesEnv, "")
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
  mode: base-mode
`)
	writeConfigFileInDir(t, configDir, "application-dev.yaml", `
server:
  port: 8081
app:
  mode: dev-mode
`)

	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want base 8080", cfg.Server.Port)
	}
	if cfg.App.Mode != "base-mode" {
		t.Fatalf("cfg.App.Mode = %q, want base-mode", cfg.App.Mode)
	}
	if got := loader.ActiveProfiles(); len(got) != 0 {
		t.Fatalf("ActiveProfiles() = %#v, want empty", got)
	}
}

func TestLoaderMergesMultipleProfilesInEnvOrder(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
  mode: base-mode
`)
	writeConfigFileInDir(t, configDir, "application-dev.yaml", `
server:
  port: 8081
app:
  mode: dev-mode
`)
	writeConfigFileInDir(t, configDir, "application-local.yaml", `
server:
  port: 8082
`)
	t.Setenv(activeProfilesEnv, " dev, ,local ")

	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8082 {
		t.Fatalf("cfg.Server.Port = %d, want rightmost profile override 8082", cfg.Server.Port)
	}
	if cfg.App.Mode != "dev-mode" {
		t.Fatalf("cfg.App.Mode = %q, want inherited profile value dev-mode", cfg.App.Mode)
	}
	if cfg.App.Name != "base" {
		t.Fatalf("cfg.App.Name = %q, want inherited base", cfg.App.Name)
	}
	if got, want := loader.ActiveProfiles(), []string{"dev", "local"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ActiveProfiles() = %#v, want %#v", got, want)
	}
}

func TestLoaderExplicitProfilesOverrideEnv(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
  mode: base-mode
`)
	writeConfigFileInDir(t, configDir, "application-prod.yaml", `
app:
  mode: prod-mode
`)
	writeConfigFileInDir(t, configDir, "application-test.yaml", `
app:
  mode: test-mode
`)
	t.Setenv(activeProfilesEnv, "prod")

	loader := NewLoader(WithConfigPaths(configDir), WithProfiles("test"))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Mode != "test-mode" {
		t.Fatalf("cfg.App.Mode = %q, want explicit profile test-mode", cfg.App.Mode)
	}
	if got, want := loader.ActiveProfiles(), []string{"test"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ActiveProfiles() = %#v, want %#v", got, want)
	}
}

func TestLoaderActiveProfilesReturnsCopy(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	t.Setenv(activeProfilesEnv, "dev,local")

	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	profiles := loader.ActiveProfiles()
	profiles[0] = "mutated"

	if got, want := loader.ActiveProfiles(), []string{"dev", "local"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ActiveProfiles() = %#v, want defensive copy %#v", got, want)
	}
}
