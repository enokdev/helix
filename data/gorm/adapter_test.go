package gorm

import (
	"testing"
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
