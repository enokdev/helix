package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type loaderTestConfig struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	App struct {
		Name string `mapstructure:"name"`
		Mode string `mapstructure:"mode"`
	} `mapstructure:"app"`
	Feature struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"feature"`
}

func TestLoaderLoadsApplicationYAMLAndDecodesStruct(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: helix
feature:
  enabled: true
`)

	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.App.Name != "helix" {
		t.Fatalf("cfg.App.Name = %q, want helix", cfg.App.Name)
	}
	if !cfg.Feature.Enabled {
		t.Fatal("cfg.Feature.Enabled should be true")
	}
	if loader.ConfigFileUsed() != filepath.Join(configDir, "application.yaml") {
		t.Fatalf("ConfigFileUsed() = %q, want %q", loader.ConfigFileUsed(), filepath.Join(configDir, "application.yaml"))
	}
}

func TestLoaderPriorityDefaultsBaseProfileAndEnv(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
feature:
  enabled: false
`)
	writeConfigFileInDir(t, configDir, "application-dev.yaml", `
server:
  port: 8081
app:
  name: dev
`)
	t.Setenv("SERVER_PORT", "9090")

	loader := NewLoader(
		WithConfigPaths(configDir),
		WithDefaults(map[string]any{
			"server.port":     7070,
			"app.mode":        "default-mode",
			"feature.enabled": true,
		}),
		WithProfiles("dev"),
	)

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want ENV override 9090", cfg.Server.Port)
	}
	if cfg.App.Name != "dev" {
		t.Fatalf("cfg.App.Name = %q, want profile override dev", cfg.App.Name)
	}
	if cfg.App.Mode != "default-mode" {
		t.Fatalf("cfg.App.Mode = %q, want default-mode", cfg.App.Mode)
	}
	if cfg.Feature.Enabled {
		t.Fatal("cfg.Feature.Enabled should inherit application.yaml false over default true")
	}

	value, ok := loader.Lookup("server.port")
	if !ok {
		t.Fatal("Lookup(server.port) not found")
	}
	if fmt.Sprint(value) != "9090" {
		t.Fatalf("Lookup(server.port) = %v, want 9090", value)
	}
}

func TestLoaderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		target    any
		wantError error
	}{
		{
			name: "missing config returns ErrConfigNotFound",
			setup: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
			target:    &loaderTestConfig{},
			wantError: ErrConfigNotFound,
		},
		{
			name: "invalid yaml returns ErrInvalidConfig",
			setup: func(t *testing.T) string {
				t.Helper()
				return writeConfigFile(t, "application.yaml", "server:\n  port: [\n")
			},
			target:    &loaderTestConfig{},
			wantError: ErrInvalidConfig,
		},
		{
			name: "invalid target returns ErrInvalidConfig",
			setup: func(t *testing.T) string {
				t.Helper()
				return writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
			},
			target:    loaderTestConfig{},
			wantError: ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loader := NewLoader(WithConfigPaths(tt.setup(t)))
			err := loader.Load(tt.target)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Load() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestLoaderMissingProfileIsSkipped(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")

	loader := NewLoader(WithConfigPaths(configDir), WithProfiles("nonexistent"))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() with missing profile should not error, got: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want 8080", cfg.Server.Port)
	}
}

func TestLoaderAllowMissingConfigLoadsProfilesDefaultsAndEnv(t *testing.T) {
	configDir := writeConfigFile(t, "application-test.yaml", `
server:
  port: 8081
app:
  name: test
`)
	t.Setenv("SERVER_PORT", "9090")

	loader := NewLoader(
		WithConfigPaths(configDir),
		WithAllowMissingConfig(),
		WithProfiles("test"),
		WithDefaults(map[string]any{"app.mode": "default-mode"}),
	)

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want ENV override 9090", cfg.Server.Port)
	}
	if cfg.App.Name != "test" {
		t.Fatalf("cfg.App.Name = %q, want test", cfg.App.Name)
	}
	if cfg.App.Mode != "default-mode" {
		t.Fatalf("cfg.App.Mode = %q, want default-mode", cfg.App.Mode)
	}
	if loader.ConfigFileUsed() != "" {
		t.Fatalf("ConfigFileUsed() = %q, want empty without application.yaml", loader.ConfigFileUsed())
	}
}

func TestLoaderAllSettingsDeepCopy(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	settings := loader.AllSettings()
	if nested, ok := settings["server"].(map[string]any); ok {
		nested["port"] = 99999
	}

	fresh := loader.AllSettings()
	if nested, ok := fresh["server"].(map[string]any); ok {
		if nested["port"] == 99999 {
			t.Fatal("AllSettings() nested map mutation should not affect loader state")
		}
	}
}

func TestLoaderAllSettingsReturnsCopy(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	settings := loader.AllSettings()
	settings["server"] = "mutated"

	fresh := loader.AllSettings()
	if fmt.Sprint(fresh["server"]) == "mutated" {
		t.Fatal("AllSettings() should return a defensive copy")
	}
}

func TestLoaderLoadsHelixLoggingLevelFromEnv(t *testing.T) {
	t.Setenv("HELIX_LOGGING_LEVEL", "debug")

	loader := NewLoader(WithAllowMissingConfig())
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	val, ok := loader.Lookup("helix.logging.level")
	if !ok {
		t.Fatal("Lookup(helix.logging.level) not found; HELIX_LOGGING_LEVEL env should be visible")
	}
	if val != "debug" {
		t.Errorf("helix.logging.level = %v, want debug", val)
	}
}

func TestLoaderLoadsHelixLoggingLevelsWebFromEnv(t *testing.T) {
	t.Setenv("HELIX_LOGGING_LEVELS_WEB", "debug")

	loader := NewLoader(WithAllowMissingConfig())
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	val, ok := loader.Lookup("helix.logging.levels.web")
	if !ok {
		t.Fatal("Lookup(helix.logging.levels.web) not found; HELIX_LOGGING_LEVELS_WEB env should be visible")
	}
	if val != "debug" {
		t.Errorf("helix.logging.levels.web = %v, want debug", val)
	}
}

func TestLoaderLoadsHelixLoggingLevelsFromYAML(t *testing.T) {
	t.Parallel()

	configDir := writeConfigFile(t, "application.yaml", `
helix:
  logging:
    level: warn
    levels:
      web: debug
`)

	loader := NewLoader(WithConfigPaths(configDir))
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	level, ok := loader.Lookup("helix.logging.level")
	if !ok {
		t.Fatal("Lookup(helix.logging.level) not found")
	}
	if level != "warn" {
		t.Errorf("helix.logging.level = %v, want warn", level)
	}

	all := loader.AllSettings()
	helixMap, ok := all["helix"].(map[string]any)
	if !ok {
		t.Fatal("all[helix] is not a map")
	}
	loggingMap, ok := helixMap["logging"].(map[string]any)
	if !ok {
		t.Fatal("helix[logging] is not a map")
	}
	levelsMap, ok := loggingMap["levels"].(map[string]any)
	if !ok {
		t.Fatal("helix.logging[levels] is not a map")
	}
	if levelsMap["web"] != "debug" {
		t.Errorf("helix.logging.levels.web = %v, want debug", levelsMap["web"])
	}
}

func writeConfigFile(t *testing.T, name, contents string) string {
	t.Helper()

	configDir := filepath.Join(t.TempDir(), "config")
	writeConfigFileInDir(t, configDir, name, contents)
	return configDir
}

func writeConfigFileInDir(t *testing.T, configDir, name, contents string) {
	t.Helper()

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
