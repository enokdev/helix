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
	Method         string   // HTTP method (GET, POST, etc.)
	Path           string   // URL path
	MethodName     string   // Go method name
	ControllerName string   // Go type name of the controller
	Guards         []string // Names of guards to apply
	Interceptors   []string // Names of interceptors to apply
	LineNum        int      // Source line number for debugging
}

// ErrorHandlerDirective represents a parsed //helix:handles directive.
type ErrorHandlerDirective struct {
	ErrorTypes     []string // Error type names
	MethodName     string   // Go method name
	ControllerName string   // Go type name of the handler
	LineNum        int      // Source line number for debugging
}

// WebScanner scans Go source files for web layer directives.
type WebScanner struct {
	fset *token.FileSet
}

// NewWebScanner creates a new WebScanner.
func NewWebScanner() *WebScanner {
	return &WebScanner{
		fset: token.NewFileSet(),
	}
}

// ScanControllerDirectives finds all web directives in a source file.
func (ws *WebScanner) ScanControllerDirectives(filename string) ([]RouteDirective, error) {
	parsed, err := parser.ParseFile(ws.fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("web_scanner: parse file %s: %w", filename, err)
	}

	var directives []RouteDirective
	for _, decl := range parsed.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		controllerName := getReceiverTypeName(funcDecl.Recv)
		if controllerName == "" {
			continue
		}

		var guards []string
		var interceptors []string

		for _, comment := range funcDecl.Doc.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))

			if strings.HasPrefix(text, "helix:route") {
				method, path, err := parseRouteDirective(comment.Text)
				if err != nil {
					return nil, fmt.Errorf("web_scanner: invalid route directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
				}

				directives = append(directives, RouteDirective{
					Method:         method,
					Path:           path,
					MethodName:     funcDecl.Name.Name,
					ControllerName: controllerName,
					Guards:         append([]string(nil), guards...),       // Copy current guards
					Interceptors:   append([]string(nil), interceptors...), // Copy current interceptors
					LineNum:        ws.fset.Position(comment.Slash).Line,
				})
			} else if strings.HasPrefix(text, "helix:guard") {
				guard, err := parseNamedDirective(comment.Text, "helix:guard")
				if err != nil {
					return nil, fmt.Errorf("web_scanner: invalid guard directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
				}
				guards = append(guards, guard)
			} else if strings.HasPrefix(text, "helix:interceptor") {
				interceptor, err := parseNamedDirective(comment.Text, "helix:interceptor")
				if err != nil {
					return nil, fmt.Errorf("web_scanner: invalid interceptor directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
				}
				interceptors = append(interceptors, interceptor)
			}
		}
	}

	return directives, nil
}

// ScanErrorHandlerDirectives finds all //helix:handles directives in a source file.
func (ws *WebScanner) ScanErrorHandlerDirectives(filename string) ([]ErrorHandlerDirective, error) {
	parsed, err := parser.ParseFile(ws.fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("web_scanner: parse file %s: %w", filename, err)
	}

	var directives []ErrorHandlerDirective
	for _, decl := range parsed.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		controllerName := getReceiverTypeName(funcDecl.Recv)
		if controllerName == "" {
			continue
		}

		for _, comment := range funcDecl.Doc.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if !strings.HasPrefix(text, "helix:handles") {
				continue
			}

			errorTypes, err := parseHandlesDirective(comment.Text)
			if err != nil {
				return nil, fmt.Errorf("web_scanner: invalid handles directive in %s:%d: %w", filename, ws.fset.Position(comment.Slash).Line, err)
			}

			directives = append(directives, ErrorHandlerDirective{
				ErrorTypes:     errorTypes,
				MethodName:     funcDecl.Name.Name,
				ControllerName: controllerName,
				LineNum:        ws.fset.Position(comment.Slash).Line,
			})
		}
	}

	return directives, nil
}

func getReceiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	t := recv.List[0].Type
	// Recurse through pointers
	for {
		if star, ok := t.(*ast.StarExpr); ok {
			t = star.X
			continue
		}
		break
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// parseRouteDirective parses a helix:route directive and returns method and path.
// Expects format: //helix:route METHOD /path
func parseRouteDirective(text string) (method, path string, err error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
	text = strings.TrimPrefix(text, "helix:route")
	text = strings.TrimSpace(text)

	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid route directive: expected METHOD /path, got %q", text)
	}

	method = strings.ToUpper(parts[0])
	path = parts[1]

	if !strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("invalid route path: %q must start with /", path)
	}

	return method, path, nil
}

// parseHandlesDirective parses a helix:handles directive and returns error type names.
// Expects format: //helix:handles ErrorType1,ErrorType2,...
func parseHandlesDirective(text string) ([]string, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
	text = strings.TrimPrefix(text, "helix:handles")
	text = strings.TrimSpace(text)

	if text == "" {
		return nil, fmt.Errorf("empty handles directive")
	}

	parts := strings.Split(text, ",")
	var errorTypes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("empty error type in handles directive: %q", text)
		}
		errorTypes = append(errorTypes, p)
	}

	return errorTypes, nil
}

// parseNamedDirective parses a directive with a single argument.
// Expects format: //helix:name arg
func parseNamedDirective(text, prefix string) (string, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
	text = strings.TrimPrefix(text, prefix)
	text = strings.TrimSpace(text)

	if text == "" {
		return "", fmt.Errorf("empty %s directive", prefix)
	}

	// Only take the first field to allow trailing comments
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty %s directive after trim", prefix)
	}

	return parts[0], nil
}
