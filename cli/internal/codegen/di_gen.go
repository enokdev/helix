package codegen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

var errCyclicWireDependency = errors.New("cyclic dependency")

// DIGenerator creates compile-time dependency wiring for Helix components.
type DIGenerator struct {
	dir string
}

type injectableComponent struct {
	PackageName  string
	TypeName     string
	Dir          string
	ImportPath   string
	Key          string
	VariableName string
	pkg          *packageModel
	Fields       []injectableField
	Dependencies []string
}

type injectableField struct {
	Name         string
	Type         string
	expr         ast.Expr
	Dependency   string
	DependencyID string
}

type wireImport struct {
	Alias string
	Path  string
}

// NewDIGenerator creates a DI wiring generator rooted at dir.
func NewDIGenerator(dir string) *DIGenerator {
	return &DIGenerator{dir: dir}
}

// Generate scans Helix components and writes helix_wire_gen.go when needed.
func (g *DIGenerator) Generate(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("cli/codegen: wire: nil context: %w", errInvalidPackage)
	}
	root := g.dir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("cli/codegen: wire: resolve root %s: %w", root, err)
	}

	var packages []*packageModel
	var targetPackage *packageModel
	if err := scanPackages(ctx, absRoot, func(pkg *packageModel) error {
		packages = append(packages, pkg)
		if filepath.Clean(pkg.dir) == filepath.Clean(absRoot) {
			targetPackage = pkg
		}
		return nil
	}); err != nil {
		return fmt.Errorf("cli/codegen: wire: scan %s: %w", root, err)
	}

	components, err := collectAllInjectableComponents(packages)
	if err != nil {
		return err
	}
	if len(components) == 0 {
		return nil
	}
	if targetPackage == nil {
		return fmt.Errorf("cli/codegen: wire: target package %s not found: %w", root, errInvalidPackage)
	}

	moduleRoot, modulePath, err := findGoModule(absRoot)
	if err != nil {
		return err
	}
	if err := completeWireComponentMetadata(components, moduleRoot, modulePath); err != nil {
		return err
	}
	assignWireVariableNames(components)
	if err := resolveWireDependencies(components); err != nil {
		return err
	}

	ordered, err := topologicalWireOrder(components)
	if err != nil {
		return err
	}

	targetImportPath, err := importPathForDir(moduleRoot, modulePath, targetPackage.dir)
	if err != nil {
		return err
	}
	content, err := renderWireFile(targetPackage.name, targetImportPath, ordered)
	if err != nil {
		return err
	}
	formatted, err := format.Source(content)
	if err != nil {
		return fmt.Errorf("cli/codegen: wire: format generated file: %w", err)
	}
	return writeFileIfChanged(filepath.Join(absRoot, "helix_wire_gen.go"), formatted)
}

func collectAllInjectableComponents(packages []*packageModel) ([]injectableComponent, error) {
	var components []injectableComponent
	for _, pkg := range packages {
		pkgComponents, err := collectInjectableComponents(pkg)
		if err != nil {
			return nil, err
		}
		components = append(components, pkgComponents...)
	}
	sort.SliceStable(components, func(i, j int) bool {
		if components[i].Dir != components[j].Dir {
			return components[i].Dir < components[j].Dir
		}
		return components[i].TypeName < components[j].TypeName
	})
	return components, nil
}

func collectInjectableComponents(pkg *packageModel) ([]injectableComponent, error) {
	componentSet := make(map[string]struct{})
	for _, component := range scanPackageComponents(pkg) {
		componentSet[component.TypeName] = struct{}{}
	}
	if len(componentSet) == 0 {
		return nil, nil
	}

	var components []injectableComponent
	for _, file := range pkg.files {
		for _, decl := range file.Decls {
			generalDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range generalDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := componentSet[typeSpec.Name.Name]; !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || structType.Fields == nil {
					continue
				}
				fields, err := collectInjectableFields(structType)
				if err != nil {
					return nil, err
				}
				components = append(components, injectableComponent{
					PackageName: pkg.name,
					TypeName:    typeSpec.Name.Name,
					Dir:         pkg.dir,
					pkg:         pkg,
					Fields:      fields,
				})
			}
		}
	}
	return components, nil
}

func collectInjectableFields(structType *ast.StructType) ([]injectableField, error) {
	var fields []injectableField
	for _, field := range structType.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		if tag.Get("inject") != "true" {
			continue
		}
		if len(field.Names) == 0 {
			return nil, fmt.Errorf("cli/codegen: wire: anonymous embedded fields with inject:\"true\" are not supported; use a named field: %w", errInvalidPackage)
		}
		for _, name := range field.Names {
			fields = append(fields, injectableField{
				Name: name.Name,
				Type: exprString(field.Type),
				expr: field.Type,
			})
		}
	}
	return fields, nil
}

func completeWireComponentMetadata(components []injectableComponent, moduleRoot, modulePath string) error {
	for i := range components {
		importPath, err := importPathForDir(moduleRoot, modulePath, components[i].Dir)
		if err != nil {
			return err
		}
		components[i].ImportPath = importPath
		components[i].Key = componentWireKey(importPath, components[i].TypeName)
	}
	return nil
}

func importPathForDir(moduleRoot, modulePath, dir string) (string, error) {
	rel, err := filepath.Rel(moduleRoot, dir)
	if err != nil {
		return "", fmt.Errorf("cli/codegen: wire: import path for %s: %w", dir, err)
	}
	if rel == "." {
		return modulePath, nil
	}
	return modulePath + "/" + filepath.ToSlash(rel), nil
}

func findGoModule(start string) (string, string, error) {
	dir := filepath.Clean(start)
	for {
		path := filepath.Join(dir, "go.mod")
		content, err := os.ReadFile(path)
		if err == nil {
			modulePath := parseModulePath(string(content))
			if modulePath == "" {
				return "", "", fmt.Errorf("cli/codegen: wire: module path missing in %s: %w", path, errInvalidPackage)
			}
			return dir, modulePath, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("cli/codegen: wire: read %s: %w", path, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("cli/codegen: wire: go.mod not found - run helix generate wire from a Go module root: %w", errInvalidPackage)
		}
		dir = parent
	}
}

func parseModulePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1]
		}
	}
	return ""
}

// goReservedKeywords is the set of Go keywords that cannot be used as identifiers.
var goReservedKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

func assignWireVariableNames(components []injectableComponent) {
	counts := make(map[string]int, len(components))
	usedNames := make(map[string]bool, len(components))
	for i := range components {
		base := lowerFirst(components[i].TypeName)
		if base == "" || goReservedKeywords[base] || usedNames[base] {
			base = lowerFirst(components[i].PackageName) + components[i].TypeName
		}
		name := base
		if usedNames[name] {
			counts[base]++
			name = fmt.Sprintf("%s%d", base, counts[base]+1)
		}
		usedNames[name] = true
		counts[base]++
		components[i].VariableName = name
	}
}

func resolveWireDependencies(components []injectableComponent) error {
	byKey := make(map[string]*injectableComponent, len(components))
	for i := range components {
		byKey[components[i].Key] = &components[i]
	}

	for i := range components {
		component := &components[i]
		for j := range component.Fields {
			field := &component.Fields[j]
			dependencyKey, ok := dependencyWireKey(component.pkg, component.ImportPath, field.expr)
			if !ok {
				return fmt.Errorf("cli/codegen: wire: unsupported dependency type %s on %s.%s: %w", field.Type, component.TypeName, field.Name, errInvalidPackage)
			}
			dependency, ok := byKey[dependencyKey]
			if !ok {
				return fmt.Errorf("cli/codegen: wire: dependency %s for %s.%s not found — interface-typed inject fields are not supported in wire mode, use a concrete pointer type: %w", field.Type, component.TypeName, field.Name, errInvalidPackage)
			}
			field.Dependency = dependency.VariableName
			field.DependencyID = dependencyKey
			component.Dependencies = append(component.Dependencies, dependencyKey)
		}
	}
	return nil
}

func dependencyWireKey(pkg *packageModel, currentImportPath string, expr ast.Expr) (string, bool) {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch node := expr.(type) {
	case *ast.Ident:
		return componentWireKey(currentImportPath, node.Name), true
	case *ast.SelectorExpr:
		ident, ok := node.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		importPath, ok := pkg.imports[ident.Name]
		if !ok {
			return "", false
		}
		return componentWireKey(importPath, node.Sel.Name), true
	default:
		return "", false
	}
}

func componentWireKey(importPath, typeName string) string {
	return importPath + "." + typeName
}

func topologicalWireOrder(components []injectableComponent) ([]injectableComponent, error) {
	byKey := make(map[string]injectableComponent, len(components))
	for _, component := range components {
		byKey[component.Key] = component
	}

	state := make(map[string]int, len(components))
	var stack []string
	ordered := make([]injectableComponent, 0, len(components))

	var visit func(string) error
	visit = func(key string) error {
		switch state[key] {
		case 1:
			return wireCycleError(stack, key)
		case 2:
			return nil
		}

		component, ok := byKey[key]
		if !ok {
			return fmt.Errorf("cli/codegen: wire: component key %q not found in dependency graph: %w", key, errInvalidPackage)
		}
		state[key] = 1
		stack = append(stack, key)
		for _, dependency := range component.Dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[key] = 2
		ordered = append(ordered, component)
		return nil
	}

	keys := make([]string, 0, len(components))
	for _, component := range components {
		keys = append(keys, component.Key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := visit(key); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func wireCycleError(stack []string, repeated string) error {
	start := 0
	for i, key := range stack {
		if key == repeated {
			start = i
			break
		}
	}
	path := append(append([]string(nil), stack[start:]...), repeated)
	return fmt.Errorf("cli/codegen: wire: cyclic dependency: %s: %w", strings.Join(path, " -> "), errCyclicWireDependency)
}

func renderWireFile(packageName, targetImportPath string, components []injectableComponent) ([]byte, error) {
	imports := wireImports(targetImportPath, components)
	aliases := make(map[string]string, len(imports))
	for _, item := range imports {
		aliases[item.Path] = item.Alias
	}

	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "// Code generated by helix generate. DO NOT EDIT.\n\n")
	fmt.Fprintf(&buffer, "package %s\n\n", packageName)
	fmt.Fprintf(&buffer, "import (\n")
	fmt.Fprintf(&buffer, "\t\"github.com/enokdev/helix\"\n")
	fmt.Fprintf(&buffer, "\t\"github.com/enokdev/helix/core\"\n")
	for _, item := range imports {
		fmt.Fprintf(&buffer, "\t%s %q\n", item.Alias, item.Path)
	}
	fmt.Fprintf(&buffer, ")\n\n")

	fmt.Fprintf(&buffer, "func init() {\n")
	fmt.Fprintf(&buffer, "\thelix.RegisterWireSetup(func(c *core.Container) error {\n")
	for _, component := range components {
		renderWireInstantiation(&buffer, component, targetImportPath, aliases)
	}
	for _, component := range components {
		fmt.Fprintf(&buffer, "\t\tif err := c.Register(%s); err != nil {\n", component.VariableName)
		fmt.Fprintf(&buffer, "\t\t\treturn err\n")
		fmt.Fprintf(&buffer, "\t\t}\n")
	}
	fmt.Fprintf(&buffer, "\t\treturn nil\n")
	fmt.Fprintf(&buffer, "\t})\n")
	fmt.Fprintf(&buffer, "}\n")
	return buffer.Bytes(), nil
}

func wireImports(targetImportPath string, components []injectableComponent) []wireImport {
	used := make(map[string]bool)
	byPath := make(map[string]wireImport)
	for _, component := range components {
		if component.ImportPath == targetImportPath {
			continue
		}
		if component.ImportPath == "github.com/enokdev/helix" || component.ImportPath == "github.com/enokdev/helix/core" {
			continue
		}
		if _, exists := byPath[component.ImportPath]; exists {
			continue
		}
		base := sanitizeImportAlias(component.PackageName) + "Pkg"
		alias := base
		for n := 2; used[alias]; n++ {
			alias = fmt.Sprintf("%s%d", base, n)
		}
		used[alias] = true
		byPath[component.ImportPath] = wireImport{
			Alias: alias,
			Path:  component.ImportPath,
		}
	}

	imports := make([]wireImport, 0, len(byPath))
	for _, item := range byPath {
		imports = append(imports, item)
	}
	sort.Slice(imports, func(i, j int) bool {
		return imports[i].Path < imports[j].Path
	})
	return imports
}

func sanitizeImportAlias(name string) string {
	var builder strings.Builder
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			builder.WriteByte('_')
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "component"
	}
	return builder.String()
}

func renderWireInstantiation(buffer *bytes.Buffer, component injectableComponent, targetImportPath string, aliases map[string]string) {
	typeExpr := wireComponentTypeExpr(component, targetImportPath, aliases)
	fmt.Fprintf(buffer, "\t\t%s := &%s{", component.VariableName, typeExpr)
	for i, field := range component.Fields {
		if i > 0 {
			fmt.Fprintf(buffer, ", ")
		}
		fmt.Fprintf(buffer, "%s: %s", field.Name, field.Dependency)
	}
	fmt.Fprintf(buffer, "}\n")
}

func wireComponentTypeExpr(component injectableComponent, targetImportPath string, aliases map[string]string) string {
	if component.ImportPath == targetImportPath {
		return component.TypeName
	}
	return aliases[component.ImportPath] + "." + component.TypeName
}
