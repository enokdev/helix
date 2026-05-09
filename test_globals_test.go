package helix

import (
	"os"
	"sync"
	"testing"
)

var cwdMu sync.Mutex

func chdirForTest(tb testing.TB, dir string) {
	tb.Helper()

	cwdMu.Lock()
	oldDir, err := os.Getwd()
	if err != nil {
		cwdMu.Unlock()
		tb.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		cwdMu.Unlock()
		tb.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	tb.Cleanup(func() {
		defer cwdMu.Unlock()
		if err := os.Chdir(oldDir); err != nil {
			tb.Fatalf("restore cwd %q: %v", oldDir, err)
		}
	})
}
