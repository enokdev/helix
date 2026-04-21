package starterutil_test

import (
	"testing"

	"github.com/enokdev/helix/starter/internal/starterutil"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		wantVal   bool
		wantParsed bool
	}{
		{"bool true", true, true, true},
		{"bool false", false, false, true},
		{"string true", "true", true, true},
		{"string True", "True", true, true},
		{"string false", "false", false, true},
		{"string 1", "1", true, true},
		{"string 0", "0", false, true},
		{"string yes", "yes", true, true},
		{"string no", "no", false, true},
		{"string unparsable", "maybe", false, false},
		{"string empty", "", false, false},
		{"float64 1.0", float64(1), true, true},
		{"float64 0.0", float64(0), false, true},
		{"float32 1.0", float32(1), true, true},
		{"float32 0.0", float32(0), false, true},
		{"int 1", int(1), true, true},
		{"int 0", int(0), false, true},
		{"int64 1", int64(1), true, true},
		{"uint 1", uint(1), true, true},
		{"uint 0", uint(0), false, true},
		{"nil", nil, false, false},
		{"struct", struct{}{}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotParsed := starterutil.ParseBool(tt.value)
			if gotParsed != tt.wantParsed {
				t.Fatalf("ParseBool(%v) parsed = %v, want %v", tt.value, gotParsed, tt.wantParsed)
			}
			if gotParsed && gotVal != tt.wantVal {
				t.Fatalf("ParseBool(%v) value = %v, want %v", tt.value, gotVal, tt.wantVal)
			}
		})
	}
}
