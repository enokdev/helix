package core

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestNewContainer_DefaultLifecycleSettings(t *testing.T) {
	t.Parallel()

	container := NewContainer()

	if container.shutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("shutdownTimeout = %v, want %v", container.shutdownTimeout, DefaultShutdownTimeout)
	}
	if container.logger == nil {
		t.Fatal("logger should be initialized by default")
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	t.Parallel()

	container := NewContainer(WithShutdownTimeout(5 * time.Second))

	if container.shutdownTimeout != 5*time.Second {
		t.Fatalf("shutdownTimeout = %v, want %v", container.shutdownTimeout, 5*time.Second)
	}
}

func TestWithShutdownTimeout_NonPositivePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("WithShutdownTimeout(0) should panic")
		}
	}()

	WithShutdownTimeout(0)
}

func TestWithLogger(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	container := NewContainer(WithLogger(logger))

	if container.logger != logger {
		t.Fatal("WithLogger did not set the logger on the container")
	}
}

func TestWithLogger_NilPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("WithLogger(nil) should panic")
		}
	}()

	WithLogger(nil)
}
