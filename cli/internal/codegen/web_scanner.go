package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// RouteDirective represents a parsed //helix:route directive.
type RouteDirective struct {
	Method     string // HTTP method (GET, POST, etc.)
	Path       string // URL path
	MethodName string // Go method name
	LineNum    int    // Source line number for debugging
}

// ErrorHandlerDirective represents a parsed //helix:handles directive.
type ErrorHandlerDirective struct {
	ErrorTypes []string // Error type names
	MethodName string   // Go method name
	LineNum    int      // Source line number for debugging
}

// WebScanner scans Go source files for web layer directives.
type WebScanner struct {
	fset *token.FileSet
}

// NewWebScanner creates a new WebScanner instance.
func NewWebScanner() *WebScanner {
	return &WebScanner{
		fset: token.NewFileSet(),
	}
}

// ScanControllerDirectives finds all //helix:route directives in a source file.
func (ws *WebScanner) ScanControllerDirectives(filename string) ([]RouteDirective, error) {
	parsed, err := parser.ParseFile(ws.fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("web_scanner: parse %s: %w", filename, err)
	}

	var directives []RouteDirective

	for _, decl := range parsed.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Doc == nil {
			continue
		}

		for _, comment := range funcDecl.Doc.List {
			text := comment.Text
			if !strings.HasPrefix(text, "//helix:route") {
				continue
			}

			method, path, err := parseRouteDirective(text)
			if err != nil {
				return nil, fmt.Errorf("web_scanner: invalid route directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
			}

			directives = append(directives, RouteDirective{
				Method:     method,
				Path:       path,
				MethodName: funcDecl.Name.Name,
				LineNum:    ws.fset.Position(comment.Slash).Line,
			})
		}
	}

	return directives, nil
}

// ScanErrorHandlerDirectives finds all //helix:handles directives in a source file.
func (ws *WebScanner) ScanErrorHandlerDirectives(filename string) ([]ErrorHandlerDirective, error) {
	parsed, err := parser.ParseFile(ws.fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("web_scanner: parse %s: %w", filename, err)
	}

	var directives []ErrorHandlerDirective

	for _, decl := range parsed.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Doc == nil {
			continue
		}

		for _, comment := range funcDecl.Doc.List {
			text := comment.Text
			if !strings.HasPrefix(text, "//helix:handles") {
				continue
			}

			errorTypes, err := parseHandlesDirective(text)
			if err != nil {
				return nil, fmt.Errorf("web_scanner: invalid handles directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
			}

			directives = append(directives, ErrorHandlerDirective{
				ErrorTypes: errorTypes,
				MethodName: funcDecl.Name.Name,
				LineNum:    ws.fset.Position(comment.Slash).Line,
			})
		}
	}

	return directives, nil
}

// parseRouteDirective parses a helix:route directive and returns method and path.
// Expects format: helix:route METHOD /path (after // has been trimmed)
func parseRouteDirective(text string) (method, path string, err error) {
	// Format: helix:route METHOD /path
	text = strings.TrimPrefix(text, "helix:route")
	text = strings.TrimSpace(text)

	parts := strings.SplitN(text, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid route directive format, expected: //helix:route METHOD /path")
	}

	method = strings.ToUpper(strings.TrimSpace(parts[0]))
	path = strings.TrimSpace(parts[1])

	if method == "" {
		return "", "", fmt.Errorf("empty HTTP method")
	}
	if path == "" || path[0] != '/' {
		return "", "", fmt.Errorf("invalid path, must start with /")
	}

	return method, path, nil
}

// parseHandlesDirective parses a helix:handles directive and returns error type names.
// Expects format: helix:handles ErrorType1,ErrorType2,... (after // has been trimmed)
func parseHandlesDirective(text string) ([]string, error) {
	// Format: helix:handles ErrorType1,ErrorType2,...
	text = strings.TrimPrefix(text, "helix:handles")
	text = strings.TrimSpace(text)

	if text == "" {
		return nil, fmt.Errorf("empty error types")
	}

	errorTypes := strings.Split(text, ",")
	for i, et := range errorTypes {
		errorTypes[i] = strings.TrimSpace(et)
		if errorTypes[i] == "" {
			return nil, fmt.Errorf("empty error type in handles directive")
		}
	}

	return errorTypes, nil
}
