package codegen

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

// OpenAPIGenerator creates a minimal OpenAPI document from Helix controllers.
type OpenAPIGenerator struct {
	dir    string
	output string
}

// NewOpenAPIGenerator creates an OpenAPI generator rooted at dir.
func NewOpenAPIGenerator(dir, output string) *OpenAPIGenerator {
	return &OpenAPIGenerator{dir: dir, output: output}
}

// Generate scans controllers and writes an OpenAPI 3 document.
func (g *OpenAPIGenerator) Generate(ctx context.Context) (GenerateResult, error) {
	if ctx == nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: openapi generate: nil context")
	}
	root := g.dir
	if root == "" {
		root = "."
	}
	output := g.output
	if output == "" {
		output = filepath.Join(root, "openapi.json")
	}

	routes, err := scanOpenAPIRoutes(ctx, root)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: openapi generate %s: %w", root, err)
	}
	doc := buildOpenAPIDocument(routes)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: openapi encode: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return GenerateResult{}, fmt.Errorf("cli/codegen: openapi write %s: %w", output, err)
	}
	return GenerateResult{Files: []string{output}}, nil
}

type openAPIRoute struct {
	Method      string
	Path        string
	OperationID string
}

type openAPIController struct {
	Name   string
	Prefix string
}

func scanOpenAPIRoutes(ctx context.Context, root string) ([]openAPIRoute, error) {
	var routes []openAPIRoute
	err := filepath.Walk(root, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name()) || strings.HasPrefix(info.Name(), "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isSourceFile(info.Name()) {
			return nil
		}
		match, err := build.Default.MatchFile(filepath.Dir(filePath), info.Name())
		if err != nil {
			return fmt.Errorf("match build constraints %s: %w", filePath, err)
		}
		if !match {
			return nil
		}
		fileRoutes, err := scanOpenAPIRoutesInFile(filePath)
		if err != nil {
			return err
		}
		routes = append(routes, fileRoutes...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	return routes, nil
}

func scanOpenAPIRoutesInFile(filePath string) ([]openAPIRoute, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("openapi: parse %s: %w", filePath, err)
	}

	controllers := make(map[string]openAPIController)
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(typeSpec.Name.Name, "Controller") {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok || !hasHelixControllerEmbed(structType) {
				continue
			}
			prefix := controllerRoutePrefixForOpenAPI(typeSpec.Name.Name)
			if tagged := controllerRouteTag(structType); tagged != "" {
				prefix = tagged
			}
			controllers[typeSpec.Name.Name] = openAPIController{Name: typeSpec.Name.Name, Prefix: prefix}
		}
	}

	var routes []openAPIRoute
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		controllerName := getReceiverTypeName(fn.Recv)
		controller, ok := controllers[controllerName]
		if !ok {
			continue
		}
		for _, directive := range openAPIRouteDirectives(fn.Doc, fset) {
			routes = append(routes, openAPIRoute{
				Method:      directive.Method,
				Path:        openAPIPath(directive.Path),
				OperationID: controller.Name + fn.Name.Name,
			})
		}
		if method, suffix, ok := conventionalOpenAPIRoute(fn.Name.Name); ok {
			routes = append(routes, openAPIRoute{
				Method:      method,
				Path:        openAPIPath(path.Join(controller.Prefix, suffix)),
				OperationID: controller.Name + fn.Name.Name,
			})
		}
	}
	return routes, nil
}

func hasHelixControllerEmbed(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if len(field.Names) != 0 {
			continue
		}
		if selector, ok := field.Type.(*ast.SelectorExpr); ok && selector.Sel.Name == "Controller" {
			return true
		}
		if star, ok := field.Type.(*ast.StarExpr); ok {
			if selector, ok := star.X.(*ast.SelectorExpr); ok && selector.Sel.Name == "Controller" {
				return true
			}
		}
	}
	return false
}

func controllerRouteTag(structType *ast.StructType) string {
	for _, field := range structType.Fields.List {
		if len(field.Names) != 0 || field.Tag == nil {
			continue
		}
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		for _, segment := range strings.Split(tag.Get("helix"), ";") {
			route, ok := strings.CutPrefix(strings.TrimSpace(segment), "route:")
			if ok && strings.HasPrefix(route, "/") {
				return path.Clean(route)
			}
		}
	}
	return ""
}

func openAPIRouteDirectives(comments *ast.CommentGroup, fset *token.FileSet) []openAPIRoute {
	if comments == nil {
		return nil
	}
	var routes []openAPIRoute
	for _, comment := range comments.List {
		method, p, err := parseRouteDirective(comment.Text)
		if err == nil {
			routes = append(routes, openAPIRoute{Method: method, Path: p})
			continue
		}
		_ = fset
	}
	return routes
}

func conventionalOpenAPIRoute(name string) (method, suffix string, ok bool) {
	switch name {
	case "Index":
		return http.MethodGet, "/", true
	case "Show":
		return http.MethodGet, "/:id", true
	case "Create":
		return http.MethodPost, "/", true
	case "Update":
		return http.MethodPut, "/:id", true
	case "Patch":
		return http.MethodPatch, "/:id", true
	case "Delete":
		return http.MethodDelete, "/:id", true
	default:
		return "", "", false
	}
}

func controllerRoutePrefixForOpenAPI(controllerName string) string {
	base := strings.TrimSuffix(controllerName, "Controller")
	words := pascalWordsForOpenAPI(base)
	if len(words) == 0 {
		return "/"
	}
	words[len(words)-1] = pluralizeForOpenAPI(words[len(words)-1])
	return "/" + strings.Join(words, "-")
}

func pascalWordsForOpenAPI(value string) []string {
	var words []string
	runes := []rune(value)
	start := 0
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || unicode.IsUpper(runes[i]) && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	return words
}

func pluralizeForOpenAPI(word string) string {
	if strings.HasSuffix(word, "s") {
		return word + "es"
	}
	if strings.HasSuffix(word, "y") && len(word) > 1 {
		prev := word[len(word)-2]
		if !strings.ContainsRune("aeiou", rune(prev)) {
			return strings.TrimSuffix(word, "y") + "ies"
		}
	}
	return word + "s"
}

func openAPIPath(routePath string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(routePath, "/"))
	parts := strings.Split(cleaned, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func buildOpenAPIDocument(routes []openAPIRoute) map[string]any {
	paths := make(map[string]any)
	for _, route := range routes {
		pathItem, _ := paths[route.Path].(map[string]any)
		if pathItem == nil {
			pathItem = make(map[string]any)
			paths[route.Path] = pathItem
		}
		pathItem[strings.ToLower(route.Method)] = map[string]any{
			"operationId": route.OperationID,
			"responses": map[string]any{
				"200": map[string]any{"description": "OK"},
			},
		}
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "Helix API",
			"version": "0.1.0",
		},
		"paths": paths,
	}
}
