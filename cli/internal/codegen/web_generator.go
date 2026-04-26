package codegen

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WebGenerator creates compile-time route and error handler registrations for Helix web layer.
type WebGenerator struct {
	dir string
}

// NewWebGenerator creates a web route/handler generator rooted at dir.
func NewWebGenerator(dir string) *WebGenerator {
	return &WebGenerator{dir: dir}
}

// GenerateResult describes the outcome of web generation.
type GenerateResult struct {
	RoutesFile        string // Path to generated helix_web_gen.go, or "" if not generated
	ErrorHandlersFile string // Path to generated helix_error_handlers_gen.go, or "" if not generated
	FileCount         int    // Number of generated files
}

// Generate scans the directory for //helix:route and //helix:handles directives,
// then generates helix_web_gen.go with registration code.
func (g *WebGenerator) Generate(ctx context.Context) (GenerateResult, error) {
	if ctx == nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: web generate: nil context: %w", errInvalidPackage)
	}

	root := g.dir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: web generate: resolve root %s: %w", root, err)
	}

	// Scan for web directives
	scanner := NewWebScanner()
	routesByFile := make(map[string][]routeInfo)
	handlersByFile := make(map[string][]handlerInfo)

	if err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip vendor, test data, generated files
			if info.Name() == "vendor" || info.Name() == "testdata" || strings.HasPrefix(info.Name(), "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_gen.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Scan the file for directives
		routes, err := scanner.ScanControllerDirectives(path)
		if err != nil {
			// Log but don't fail on scan errors
			return nil
		}
		if len(routes) > 0 {
			routesByFile[path] = convertScannerRoutes(routes)
		}

		handlers, err := scanner.ScanErrorHandlerDirectives(path)
		if err != nil {
			return nil
		}
		if len(handlers) > 0 {
			handlersByFile[path] = convertScannerHandlers(handlers)
		}

		return nil
	}); err != nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: web generate: scan directives: %w", err)
	}

	result := GenerateResult{}

	// Generate routes file if we found any routes
	if len(routesByFile) > 0 {
		routesPath := filepath.Join(absRoot, "helix_web_gen.go")
		if err := g.generateRoutesFile(routesPath, routesByFile); err != nil {
			return GenerateResult{}, err
		}
		result.RoutesFile = routesPath
		result.FileCount++
	}

	// Generate error handlers file if we found any handlers
	if len(handlersByFile) > 0 {
		handlersPath := filepath.Join(absRoot, "helix_error_handlers_gen.go")
		if err := g.generateHandlersFile(handlersPath, handlersByFile); err != nil {
			return GenerateResult{}, err
		}
		result.ErrorHandlersFile = handlersPath
		result.FileCount++
	}

	return result, nil
}

type routeInfo struct {
	Controller  string
	HandlerName string
	Method      string
	Path        string
}

type handlerInfo struct {
	Handler     string
	MethodName  string
	ErrorType   string
}

func convertScannerRoutes(scannerRoutes []RouteDirective) []routeInfo {
	var routes []routeInfo
	for _, sr := range scannerRoutes {
		routes = append(routes, routeInfo{
			HandlerName: sr.MethodName,
			Method:      sr.Method,
			Path:        sr.Path,
		})
	}
	return routes
}

func convertScannerHandlers(scannerHandlers []ErrorHandlerDirective) []handlerInfo {
	var handlers []handlerInfo
	for _, sh := range scannerHandlers {
		// Each error handler method can handle multiple error types
		for _, errorType := range sh.ErrorTypes {
			handlers = append(handlers, handlerInfo{
				MethodName: sh.MethodName,
				ErrorType:  errorType,
			})
		}
	}
	return handlers
}

func (g *WebGenerator) generateRoutesFile(outputPath string, routesByFile map[string][]routeInfo) error {
	// Collect all unique routes with their controller names
	type routeWithController struct {
		Controller  string
		HandlerName string
		Method      string
		Path        string
	}
	var allRoutes []routeWithController
	for _, routes := range routesByFile {
		for _, route := range routes {
			allRoutes = append(allRoutes, routeWithController{
				Controller:  route.Controller,
				HandlerName: route.HandlerName,
				Method:      route.Method,
				Path:        route.Path,
			})
		}
	}

	// Sort for deterministic output
	sort.Slice(allRoutes, func(i, j int) bool {
		if allRoutes[i].Controller != allRoutes[j].Controller {
			return allRoutes[i].Controller < allRoutes[j].Controller
		}
		if allRoutes[i].HandlerName != allRoutes[j].HandlerName {
			return allRoutes[i].HandlerName < allRoutes[j].HandlerName
		}
		return allRoutes[i].Path < allRoutes[j].Path
	})

	buf := &bytes.Buffer{}
	buf.WriteString(`// Code generated by helix generate; DO NOT EDIT.

package main

import (
	"github.com/enokdev/helix/web"
	"github.com/enokdev/helix/web/internal"
)

// initGeneratedRoutes registers pre-generated routes from compile-time directive scanning.
// This function is called during app initialization to avoid runtime AST parsing.
func initGeneratedRoutes() error {
	registry := internal.GlobalRouteRegistry()

	// Register all scanned routes in the registry
`)

	// Group routes by controller
	routesByController := make(map[string][]routeWithController)
	for _, r := range allRoutes {
		routesByController[r.Controller] = append(routesByController[r.Controller], r)
	}

	// Sort controllers for deterministic output
	controllers := make([]string, 0, len(routesByController))
	for controller := range routesByController {
		controllers = append(controllers, controller)
	}
	sort.Strings(controllers)

	for _, controller := range controllers {
		routes := routesByController[controller]
		buf.WriteString(fmt.Sprintf("\t// Routes for %s\n", controller))
		buf.WriteString(fmt.Sprintf("\tif err := registry.RegisterGeneratedRoutes(\"%s\",\n", controller))

		for i, route := range routes {
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString(fmt.Sprintf("\t\tinternal.RouteInfo{\n"+
				"\t\t\tMethod:      \"%s\",\n"+
				"\t\t\tPath:        \"%s\",\n"+
				"\t\t\tController:  \"%s\",\n"+
				"\t\t\tHandlerName: \"%s\",\n"+
				"\t\t\t// Handler will be populated by router registration\n"+
				"\t\t\tHandler:     nil,\n"+
				"\t\t}",
				route.Method, route.Path, route.Controller, route.HandlerName))
		}
		buf.WriteString(",\n\t); err != nil {\n")
		buf.WriteString(fmt.Sprintf("\t\treturn err\n"))
		buf.WriteString("\t}\n\n")
	}

	buf.WriteString(`	return nil
}
`)

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("cli/codegen: format routes file: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
		return fmt.Errorf("cli/codegen: write routes file %s: %w", outputPath, err)
	}

	return nil
}

func (g *WebGenerator) generateHandlersFile(outputPath string, handlersByFile map[string][]handlerInfo) error {
	// Collect all unique handlers
	var allHandlers []handlerInfo
	for _, handlers := range handlersByFile {
		allHandlers = append(allHandlers, handlers...)
	}

	// Sort for deterministic output
	sort.Slice(allHandlers, func(i, j int) bool {
		if allHandlers[i].Handler != allHandlers[j].Handler {
			return allHandlers[i].Handler < allHandlers[j].Handler
		}
		return allHandlers[i].ErrorType < allHandlers[j].ErrorType
	})

	buf := &bytes.Buffer{}
	buf.WriteString(`// Code generated by helix generate; DO NOT EDIT.

package main

import (
	"github.com/enokdev/helix/web/internal"
)

// initGeneratedErrorHandlers registers pre-generated error handlers from compile-time directive scanning.
// This function is called during app initialization to avoid runtime AST parsing.
func initGeneratedErrorHandlers() error {
	registry := internal.GlobalErrorHandlerRegistry()

	// Register all scanned error handlers in the registry
	if err := registry.RegisterGeneratedErrorHandlers(
`)

	for i, handler := range allHandlers {
		if i > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(fmt.Sprintf("\t\tinternal.ErrorHandlerInfo{\n"+
			"\t\t\tErrorType:  \"%s\",\n"+
			"\t\t\tMethodName: \"%s\",\n"+
			"\t\t\t// Handler will be populated by error handler registration\n"+
			"\t\t\tHandler:    nil,\n"+
			"\t\t}",
			handler.ErrorType, handler.MethodName))
	}

	buf.WriteString(",\n\t); err != nil {\n")
	buf.WriteString("\t\treturn err\n")
	buf.WriteString("\t}\n\n")
	buf.WriteString("\treturn nil\n}\n")

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("cli/codegen: format handlers file: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, formatted, 0644); err != nil {
		return fmt.Errorf("cli/codegen: write handlers file %s: %w", outputPath, err)
	}

	return nil
}
