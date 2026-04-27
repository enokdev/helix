package gomodutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrGoModNotFound is returned when no go.mod file can be found in the directory tree.
var ErrGoModNotFound = errors.New("go.mod not found in directory tree")

// FindGoModPath searches for the go.mod file starting from the current working directory
// and walking up the directory tree until it finds go.mod or reaches the filesystem root.
// It returns the absolute path to the go.mod file, or ErrGoModNotFound if not found.
func FindGoModPath() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("starter: getwd: %w", err)
	}

	for {
		candidate := filepath.Join(current, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root
			return "", ErrGoModNotFound
		}
		current = parent
	}
}
