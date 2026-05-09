//go:build integration

package gorm_test

import (
	"log/slog"
	"sync"
	"testing"
)

var defaultLoggerMu sync.Mutex

func setDefaultLoggerForTest(t *testing.T, logger *slog.Logger) {
	t.Helper()

	defaultLoggerMu.Lock()
	original := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		defer defaultLoggerMu.Unlock()
		slog.SetDefault(original)
	})
}
