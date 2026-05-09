package gorm

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/enokdev/helix/data"
)

func TestEscapeLikeEscapesWildcards(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "percent", value: "50% off", want: `50\% off`},
		{name: "underscore", value: "user_name", want: `user\_name`},
		{name: "backslash", value: `c:\tmp`, want: `c:\\tmp`},
		{name: "combined", value: `50%_off\`, want: `50\%\_off\\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeLike(tt.value); got != tt.want {
				t.Fatalf("escapeLike(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestWrapErrorPreservesInvalidFilterSentinelWithoutSpecialCase(t *testing.T) {
	err := WrapError("find where", data.ErrInvalidFilter)
	if !errors.Is(err, data.ErrInvalidFilter) {
		t.Fatalf("WrapError() = %v, want data.ErrInvalidFilter", err)
	}
	if !strings.Contains(err.Error(), "data/gorm: find where") {
		t.Fatalf("WrapError() = %q, want action context", err.Error())
	}

	source, readErr := os.ReadFile("adapter.go")
	if readErr != nil {
		t.Fatalf("read adapter.go: %v", readErr)
	}
	if strings.Contains(string(source), "case errors.Is(err, data.ErrInvalidFilter)") {
		t.Fatal("wrapError must not keep an ErrInvalidFilter case identical to default")
	}
}
