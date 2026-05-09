package codegen

import (
	"os"
	"sync"
	"testing"
)

var cwdMu sync.Mutex

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	cwdMu.Lock()
	oldDir, err := os.Getwd()
	if err != nil {
		cwdMu.Unlock()
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		cwdMu.Unlock()
		t.Fatalf("os.Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		defer cwdMu.Unlock()
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd %q: %v", oldDir, err)
		}
	})
}
