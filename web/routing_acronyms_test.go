package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPascalWordsAcronymHandling validates acronym handling in camelCase to kebab-case conversion
func TestPascalWordsAcronymHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple word",
			input:    "user",
			expected: []string{"user"},
		},
		{
			name:     "camelcase",
			input:    "userName",
			expected: []string{"user", "name"},
		},
		{
			name:     "terminal acronym HTTP",
			input:    "userHTTP",
			expected: []string{"user", "http"},
		},
		{
			name:     "terminal acronym HTTPs",
			input:    "userHTTPS",
			expected: []string{"user", "https"},
		},
		{
			name:     "acronym prefix",
			input:    "HTTPSConfig",
			expected: []string{"https", "config"},
		},
		{
			name:     "mixed API and Client",
			input:    "APIClient",
			expected: []string{"api", "client"},
		},
		{
			name:     "acronym at end",
			input:    "DataID",
			expected: []string{"data", "id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pascalWords(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestControllerRoutePrefixAcronyms validates route prefix generation with acronyms
func TestControllerRoutePrefixAcronyms(t *testing.T) {
	tests := []struct {
		name              string
		controllerName    string
		expectedRoute     string
		shouldError       bool
	}{
		{
			name:           "simple controller",
			controllerName: "UserController",
			expectedRoute:  "/users",
		},
		{
			name:           "API controller",
			controllerName: "APIController",
			expectedRoute:  "/apis",
		},
		{
			name:           "APIClient controller",
			controllerName: "APIClientController",
			expectedRoute:  "/api-clients",
		},
		{
			name:           "HTTPS config controller",
			controllerName: "HTTPSConfigController",
			expectedRoute:  "/https-configs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := controllerRoutePrefix(tt.controllerName)
			
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRoute, result)
			}
		})
	}
}
