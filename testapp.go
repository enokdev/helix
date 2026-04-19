package helix

import (
	"testing"

	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/testutil"
)

// TestApp is a Helix application container scoped to a Go test.
type TestApp = testutil.App

// TestOption configures a Helix test application.
type TestOption = testutil.Option

// NewTestApp creates, starts, and registers cleanup for a Helix test app.
func NewTestApp(t testing.TB, opts ...TestOption) *TestApp {
	t.Helper()
	return testutil.NewApp(t, opts...)
}

// GetBean resolves a component from a Helix test app.
func GetBean[T any](app *TestApp) T {
	return testutil.GetBean[T](app)
}

// TestComponents registers components in the test container before startup.
func TestComponents(components ...any) TestOption {
	return testutil.WithComponents(components...)
}

// MockBean replaces components assignable to T with impl in a Helix test app.
func MockBean[T any](impl T) TestOption {
	return testutil.MockBean[T](impl)
}

// TestConfigPaths overrides directories searched for application test config.
func TestConfigPaths(paths ...string) TestOption {
	return testutil.WithConfigPaths(paths...)
}

// TestConfigDefaults configures fallback values for the test config loader.
func TestConfigDefaults(values map[string]any) TestOption {
	return testutil.WithConfigDefaults(values)
}

// TestContainerOptions appends core container options to the test container.
func TestContainerOptions(opts ...core.Option) TestOption {
	return testutil.WithContainerOptions(opts...)
}
