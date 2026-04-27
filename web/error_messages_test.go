package web_test

import (
	"strings"
	"testing"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
	"github.com/stretchr/testify/assert"
)

// TestErrorMessagesForInvalidController validates that error messages are detailed and helpful
func TestErrorMessagesForInvalidController(t *testing.T) {
	tests := []struct {
		name              string
		controller        any
		shouldContain     []string // substring expectations for error message
		shouldNotContain  []string
	}{
		{
			name:       "no public methods",
			controller: &NoPublicMethodsController{},
			shouldContain: []string{
				"NoPublicMethodsController",
				"no routable methods found",
				"add public methods",
			},
		},
		{
			name:       "no marker embed",
			controller: &NoMarkerController{},
			shouldContain: []string{
				"NoMarkerController",
				"marker",
			},
		},
		{
			name:       "invalid route tag",
			controller: &InvalidRouteTagController{},
			shouldContain: []string{
				"InvalidRouteTagController",
				"tag",
				"route",
				"illegal '..' sequence",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := web.RegisterController(newMockHTTPServer(), tt.controller)
			assert.Error(t, err)
			errMsg := err.Error()
			
			for _, substr := range tt.shouldContain {
				assert.True(t, strings.Contains(errMsg, substr),
					"error should contain %q, but got: %s", substr, errMsg)
			}
			
			for _, substr := range tt.shouldNotContain {
				assert.False(t, strings.Contains(errMsg, substr),
					"error should NOT contain %q, but got: %s", substr, errMsg)
			}
		})
	}
}

// NoPublicMethodsController has no public methods (all are unexported)
type NoPublicMethodsController struct {
	helix.Controller
}

// NoMarkerController is missing the helix.Controller embed
type NoMarkerController struct{}

func (c *NoMarkerController) Index() {}

// InvalidRouteTagController has an invalid route tag
type InvalidRouteTagController struct {
	helix.Controller `helix:"route:/../invalid"`
}

func (c *InvalidRouteTagController) Index() {}
