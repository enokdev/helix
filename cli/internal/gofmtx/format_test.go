package gofmtx

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestSourceFormatsGeneratedGo(t *testing.T) {
	t.Parallel()

	formatted, err := Source([]byte("package generated\n\nfunc Example( value string ) string { return value }\n"))
	if err != nil {
		t.Fatalf("Source() error = %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", formatted, 0); err != nil {
		t.Fatalf("formatted source does not parse: %v\n%s", err, formatted)
	}
	if strings.Contains(string(formatted), "( value string )") {
		t.Fatalf("source was not formatted:\n%s", formatted)
	}
}
