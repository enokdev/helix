package gofmtx

import (
	"fmt"

	gofumptformat "mvdan.cc/gofumpt/format"
)

// Source formats generated Go code with the same stricter formatter used by the project.
func Source(src []byte) ([]byte, error) {
	formatted, err := gofumptformat.Source(src, gofumptformat.Options{
		LangVersion: "1.21",
		ExtraRules:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("gofmtx: format source: %w", err)
	}
	return formatted, nil
}
