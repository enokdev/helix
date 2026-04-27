package gomodutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGoModPath(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, func())
		expectedErr bool
	}{
		{
			name: "go.mod in current directory",
			setup: func(t *testing.T) (string, func()) {
				tmpDir := t.TempDir()
				goModPath := filepath.Join(tmpDir, "go.mod")
				err := os.WriteFile(goModPath, []byte("module github.com/example/test\n"), 0644)
				require.NoError(t, err)

				oldCwd, err := os.Getwd()
				require.NoError(t, err)
				err = os.Chdir(tmpDir)
				require.NoError(t, err)

				cleanup := func() {
					_ = os.Chdir(oldCwd)
				}
				return tmpDir, cleanup
			},
			expectedErr: false,
		},
		{
			name: "go.mod in parent directory",
			setup: func(t *testing.T) (string, func()) {
				tmpDir := t.TempDir()
				goModPath := filepath.Join(tmpDir, "go.mod")
				err := os.WriteFile(goModPath, []byte("module github.com/example/test\n"), 0644)
				require.NoError(t, err)

				subDir := filepath.Join(tmpDir, "subdir", "nested")
				err = os.MkdirAll(subDir, 0755)
				require.NoError(t, err)

				oldCwd, err := os.Getwd()
				require.NoError(t, err)
				err = os.Chdir(subDir)
				require.NoError(t, err)

				cleanup := func() {
					_ = os.Chdir(oldCwd)
				}
				return tmpDir, cleanup
			},
			expectedErr: false,
		},
		{
			name: "no go.mod anywhere",
			setup: func(t *testing.T) (string, func()) {
				tmpDir := t.TempDir()
				subDir := filepath.Join(tmpDir, "isolated")
				err := os.MkdirAll(subDir, 0755)
				require.NoError(t, err)

				oldCwd, err := os.Getwd()
				require.NoError(t, err)
				err = os.Chdir(subDir)
				require.NoError(t, err)

				cleanup := func() {
					_ = os.Chdir(oldCwd)
				}
				return tmpDir, cleanup
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup := tt.setup(t)
			defer cleanup()

			path, err := FindGoModPath()
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, ErrGoModNotFound, err)
				assert.Equal(t, "", path)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, path)
				assert.True(t, filepath.IsAbs(path))
				assert.Equal(t, "go.mod", filepath.Base(path))
				// Verify the found go.mod exists
				_, err := os.Stat(path)
				assert.NoError(t, err, "found go.mod should exist")
			}
		})
	}
}

func TestFindGoModPathCurrentProject(t *testing.T) {
	// This test verifies that FindGoModPath works in the actual Helix repository
	path, err := FindGoModPath()
	require.NoError(t, err, "Should find go.mod in Helix repository")
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, "go.mod", filepath.Base(path))

	// Verify the file actually exists
	stat, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, stat.IsDir())
}
