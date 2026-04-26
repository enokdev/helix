package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRouteDirective(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantMethod string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "valid route directive",
			text:       "helix:route GET /users",
			wantMethod: "GET",
			wantPath:   "/users",
		},
		{
			name:       "method with lowercase",
			text:       "helix:route post /users",
			wantMethod: "POST",
			wantPath:   "/users",
		},
		{
			name:       "complex path",
			text:       "helix:route GET /api/v1/users/:id/posts",
			wantMethod: "GET",
			wantPath:   "/api/v1/users/:id/posts",
		},
		{
			name:    "missing path",
			text:    "helix:route GET",
			wantErr: true,
		},
		{
			name:    "path without leading slash",
			text:    "helix:route GET users",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, err := parseRouteDirective(tt.text)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMethod, method)
				assert.Equal(t, tt.wantPath, path)
			}
		})
	}
}

func TestParseHandlesDirective(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantTypes []string
		wantErr   bool
	}{
		{
			name:      "single error type",
			text:      "helix:handles NotFoundError",
			wantTypes: []string{"NotFoundError"},
		},
		{
			name:      "multiple error types",
			text:      "helix:handles ValidationError,BadRequestError,NotFoundError",
			wantTypes: []string{"ValidationError", "BadRequestError", "NotFoundError"},
		},
		{
			name:      "spaces around commas",
			text:      "helix:handles Error1 , Error2 , Error3",
			wantTypes: []string{"Error1", "Error2", "Error3"},
		},
		{
			name:    "empty error types",
			text:    "helix:handles",
			wantErr: true,
		},
		{
			name:    "empty error type in list",
			text:    "helix:handles Error1,,Error2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types, err := parseHandlesDirective(tt.text)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantTypes, types)
			}
		})
	}
}
