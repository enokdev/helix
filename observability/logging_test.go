package observability_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/observability"
)

// restoreDefault enregistre slog.Default() courant et le restaure après le test.
func restoreDefault(t *testing.T) {
	t.Helper()
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })
}

// decodeLogLine décode une ligne JSON de log.
func decodeLogLine(t *testing.T, line string) map[string]any {
	t.Helper()
	line = strings.TrimSpace(line)
	if line == "" {
		t.Fatal("empty log line")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", line, err)
	}
	return m
}

// firstLine retourne la première ligne non-vide du buffer.
func firstLine(buf *bytes.Buffer) string {
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// ─── ConfigureLogging defaults ───────────────────────────────────────────────

func TestConfigureLogging_DefaultsJSON(t *testing.T) {
	// slog.SetDefault est global — pas de t.Parallel()
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Info("application started")

	entry := decodeLogLine(t, firstLine(&buf))
	for _, key := range []string{"timestamp", "level", "msg", "namespace"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing key %q in JSON entry: %v", key, entry)
		}
	}
	if entry["namespace"] != "app" {
		t.Errorf("namespace = %v, want app", entry["namespace"])
	}
	if entry["msg"] != "application started" {
		t.Errorf("msg = %v, want %q", entry["msg"], "application started")
	}
}

func TestConfigureLogging_DefaultLevelIsInfo(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Debug("debug message")
	logger.Info("info message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("debug should be filtered at default info level")
	}
	if !strings.Contains(output, "info message") {
		t.Error("info should pass at default info level")
	}
}

// ─── Global level filtering ──────────────────────────────────────────────────

func TestConfigureLogging_WarnLevel_FiltersDebugAndInfo(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
		observability.WithLoggingConfig(observability.LoggingConfig{Level: "warn"}),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Debug("debug msg")
	logger.Info("info msg")
	logger.Warn("warn msg")
	logger.Error("error msg")

	output := buf.String()
	if strings.Contains(output, "debug msg") {
		t.Error("debug must be filtered at warn level")
	}
	if strings.Contains(output, "info msg") {
		t.Error("info must be filtered at warn level")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("warn must pass at warn level")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("error must pass at warn level")
	}
}

// ─── Namespace level override ────────────────────────────────────────────────

func TestConfigureLogging_NamespaceLevel_AllowsDebugForWeb(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
		observability.WithLoggingConfig(observability.LoggingConfig{
			Level:  "warn",
			Levels: map[string]string{"web": "debug"},
		}),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	webLogger := logger.With(slog.String("namespace", "web"))
	dataLogger := logger.With(slog.String("namespace", "data"))

	webLogger.Debug("web debug msg")
	dataLogger.Debug("data debug msg")
	webLogger.Warn("web warn msg")

	output := buf.String()
	if !strings.Contains(output, "web debug msg") {
		t.Error("web debug should pass with namespace override debug")
	}
	if strings.Contains(output, "data debug msg") {
		t.Error("data debug should be filtered at global warn level")
	}
	if !strings.Contains(output, "web warn msg") {
		t.Error("web warn should pass")
	}
}

// ─── slog.Default() integration ──────────────────────────────────────────────

func TestConfigureLogging_SlogDefault_ProducesHelixJSON(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	_, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	slog.Info("via default", "key", "val")

	entry := decodeLogLine(t, firstLine(&buf))
	if _, ok := entry["timestamp"]; !ok {
		t.Error("timestamp missing from slog.Default() output")
	}
	if entry["namespace"] != "app" {
		t.Errorf("namespace = %v, want app", entry["namespace"])
	}
}

// ─── Timestamp field ─────────────────────────────────────────────────────────

func TestConfigureLogging_TimestampField_NotTimeKey(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Info("ts test")

	entry := decodeLogLine(t, firstLine(&buf))
	if _, ok := entry["time"]; ok {
		t.Error("entry must not contain key 'time'; should be renamed to 'timestamp'")
	}
	if _, ok := entry["timestamp"]; !ok {
		t.Error("entry must contain key 'timestamp'")
	}
}

// ─── No duplicate namespace ──────────────────────────────────────────────────

func TestConfigureLogging_NoduplicateNamespace(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.With(slog.String("namespace", "web")).Info("no duplicate")

	raw := firstLine(&buf)
	count := strings.Count(raw, `"namespace"`)
	if count != 1 {
		t.Errorf("expected exactly 1 'namespace' key in JSON, got %d: %s", count, raw)
	}
}

// ─── Default namespace ───────────────────────────────────────────────────────

func TestConfigureLogging_DefaultNamespaceApp(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Info("default ns test")

	entry := decodeLogLine(t, firstLine(&buf))
	if entry["namespace"] != "app" {
		t.Errorf("namespace = %v, want app", entry["namespace"])
	}
}

// ─── Invalid levels ──────────────────────────────────────────────────────────

func TestConfigureLogging_InvalidLevel_ReturnsErrInvalidLogging(t *testing.T) {
	t.Parallel()

	invalidLevels := []string{"", "trace", "fatal", "verbose", "TRACE", "  ", "critical"}

	for _, lvl := range invalidLevels {
		lvl := lvl
		t.Run(lvl, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			_, err := observability.ConfigureLogging(nil,
				observability.WithLoggingOutput(&buf),
				observability.WithLoggingConfig(observability.LoggingConfig{Level: lvl}),
			)
			if err == nil {
				t.Fatalf("ConfigureLogging(level=%q) should return error", lvl)
			}
			if !errors.Is(err, observability.ErrInvalidLogging) {
				t.Errorf("errors.Is(err, ErrInvalidLogging) = false; err = %v", err)
			}
		})
	}
}

func TestConfigureLogging_InvalidNamespaceLevel_ReturnsErrInvalidLogging(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	_, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
		observability.WithLoggingConfig(observability.LoggingConfig{
			Level:  "info",
			Levels: map[string]string{"web": "verbose"},
		}),
	)
	if err == nil {
		t.Fatal("ConfigureLogging() with invalid namespace level should return error")
	}
	if !errors.Is(err, observability.ErrInvalidLogging) {
		t.Errorf("errors.Is(err, ErrInvalidLogging) = false; err = %v", err)
	}
}

// ─── Option guards ───────────────────────────────────────────────────────────

func TestConfigureLogging_NilOutput_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := observability.ConfigureLogging(nil, observability.WithLoggingOutput(nil))
	if err == nil {
		t.Fatal("nil output should return error")
	}
	if !errors.Is(err, observability.ErrInvalidLogging) {
		t.Errorf("errors.Is(err, ErrInvalidLogging) = false; err = %v", err)
	}
}

func TestConfigureLogging_EmptyNamespace_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := observability.ConfigureLogging(nil, observability.WithDefaultNamespace(""))
	if err == nil {
		t.Fatal("empty namespace should return error")
	}
	if !errors.Is(err, observability.ErrInvalidLogging) {
		t.Errorf("errors.Is(err, ErrInvalidLogging) = false; err = %v", err)
	}
}

func TestConfigureLogging_NilOption_ReturnsError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	_, err := observability.ConfigureLogging(nil, observability.WithLoggingOutput(&buf), nil)
	if err == nil {
		t.Fatal("nil option should return error")
	}
	if !errors.Is(err, observability.ErrInvalidLogging) {
		t.Errorf("errors.Is(err, ErrInvalidLogging) = false; err = %v", err)
	}
}

// ─── Logger function ─────────────────────────────────────────────────────────

func TestLogger_SetsNamespaceAttr(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	_, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	observability.Logger("myns").Info("ns test")

	entry := decodeLogLine(t, firstLine(&buf))
	if entry["namespace"] != "myns" {
		t.Errorf("namespace = %v, want myns", entry["namespace"])
	}
}

// ─── WithDefaultNamespace option ─────────────────────────────────────────────

func TestConfigureLogging_CustomDefaultNamespace(t *testing.T) {
	restoreDefault(t)

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(nil,
		observability.WithLoggingOutput(&buf),
		observability.WithDefaultNamespace("myapp"),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	logger.Info("custom default ns")

	entry := decodeLogLine(t, firstLine(&buf))
	if entry["namespace"] != "myapp" {
		t.Errorf("namespace = %v, want myapp", entry["namespace"])
	}
}

// ─── ConfigureLogging via config.Loader (YAML) ───────────────────────────────

func TestConfigureLogging_WithLoader_YAMLNestedLevels(t *testing.T) {
	restoreDefault(t)

	configDir := writeTestConfigFile(t, "application.yaml", `
helix:
  logging:
    level: warn
    levels:
      web: debug
`)

	loader := config.NewLoader(config.WithConfigPaths(configDir))
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}

	var buf bytes.Buffer
	logger, err := observability.ConfigureLogging(loader,
		observability.WithLoggingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("ConfigureLogging() error = %v", err)
	}

	webLogger := logger.With(slog.String("namespace", "web"))
	dataLogger := logger.With(slog.String("namespace", "data"))

	webLogger.Debug("web debug yaml")
	dataLogger.Debug("data debug yaml")
	webLogger.Warn("web warn yaml")

	output := buf.String()
	if !strings.Contains(output, "web debug yaml") {
		t.Error("web debug should pass with YAML namespace level debug")
	}
	if strings.Contains(output, "data debug yaml") {
		t.Error("data debug should be filtered at global warn level")
	}
	if !strings.Contains(output, "web warn yaml") {
		t.Error("web warn should pass")
	}
}

func writeTestConfigFile(t *testing.T, name, contents string) string {
	t.Helper()
	configDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return configDir
}
