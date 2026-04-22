package codegen

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const helixPackagePath = "github.com/enokdev/helix"

var errInvalidHelixDirective = errors.New("invalid helix directive")

// ComponentKind identifies the Helix marker embedded by a component struct.
type ComponentKind string

const (
	// ComponentService marks a struct embedding helix.Service.
	ComponentService ComponentKind = "service"
	// ComponentController marks a struct embedding helix.Controller.
	ComponentController ComponentKind = "controller"
	// ComponentRepository marks a struct embedding helix.Repository.
	ComponentRepository ComponentKind = "repository"
	// ComponentComponent marks a struct embedding helix.Component.
	ComponentComponent ComponentKind = "component"
)

// Scanner reads Go source packages and extracts Helix code generation metadata.
type Scanner struct {
	dir string
}

// ScanResult contains all metadata discovered in one scanner run.
type ScanResult struct {
	Packages   []PackageInfo
	Components []ComponentInfo
	Directives []DirectiveInfo
}

// PackageInfo describes one parsed Go package.
type PackageInfo struct {
	Name string
	Dir  string
}

// ComponentInfo describes one struct embedding a Helix component marker.
type ComponentInfo struct {
	Package  string
	File     string
	Line     int
	TypeName string
	Kind     ComponentKind
}

// DirectiveInfo describes one canonical Helix directive comment.
type DirectiveInfo struct {
	Package  string
	File     string
	Line     int
	Target   string
	Name     string
	Argument string
	Raw      string
}

// NewScanner creates a source scanner rooted at dir.
func NewScanner(dir string) *Scanner {
	return &Scanner{dir: dir}
}

// Scan parses the configured directory tree and returns Helix metadata.
func (s *Scanner) Scan(ctx context.Context) (ScanResult, error) {
	if ctx == nil {
		return ScanResult{}, fmt.Errorf("cli/codegen: scan: nil context")
	}
	root := s.dir
	if root == "" {
		root = "."
	}

	var result ScanResult
	if err := scanPackages(ctx, root, func(pkg *packageModel) error {
		result.Packages = append(result.Packages, PackageInfo{Name: pkg.name, Dir: pkg.dir})
		result.Components = append(result.Components, scanPackageComponents(pkg)...)
		directives, err := scanPackageDirectives(pkg)
		if err != nil {
			return err
		}
		result.Directives = append(result.Directives, directives...)
		return nil
	}); err != nil {
		return ScanResult{}, fmt.Errorf("cli/codegen: scan %s: %w", root, err)
	}

	sort.Slice(result.Components, func(i, j int) bool {
		return sourceOrder(result.Components[i].File, result.Components[i].Line, result.Components[i].TypeName, result.Components[j].File, result.Components[j].Line, result.Components[j].TypeName)
	})
	sort.Slice(result.Directives, func(i, j int) bool {
		return sourceOrder(result.Directives[i].File, result.Directives[i].Line, result.Directives[i].Target, result.Directives[j].File, result.Directives[j].Line, result.Directives[j].Target)
	})
	return result, nil
}

func scanPackages(ctx context.Context, root string, visit func(*packageModel) error) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && shouldSkipDir(entry.Name()) {
			return filepath.SkipDir
		}

		pkg, err := loadPackage(path)
		if err != nil {
			return err
		}
		if pkg == nil {
			return nil
		}
		return visit(pkg)
	})
}

func (p *packageModel) collectImports(file *ast.File) {
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		name := pathpkg.Base(importPath)
		if importSpec.Name != nil {
			name = importSpec.Name.Name
		}
		if name == "." || name == "_" {
			continue
		}
		p.imports[name] = importPath
	}
}

func (p *packageModel) typeCheck() {
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	config := types.Config{
		Importer: scannerImporter{},
		Error:    func(error) {},
	}
	_, _ = config.Check(p.name, p.fset, p.files, info)
	p.typesInfo = info
}

type scannerImporter struct{}

func (scannerImporter) Import(path string) (*types.Package, error) {
	if pkg, err := importer.Default().Import(path); err == nil {
		return pkg, nil
	}

	name := pathpkg.Base(path)
	if path == helixPackagePath {
		name = "helix"
	}
	pkg := types.NewPackage(path, name)
	if path == helixPackagePath {
		insertFakeTypeNames(pkg)
	}
	pkg.MarkComplete()
	return pkg, nil
}

func insertFakeTypeNames(pkg *types.Package) {
	names := []string{"DB", "Repository", "Service", "Controller", "Component", "ErrorHandler", "SecurityConfigurer"}
	for _, name := range names {
		obj := types.NewTypeName(token.NoPos, pkg, name, types.NewStruct(nil, nil))
		pkg.Scope().Insert(obj)
	}
}

func scanPackageComponents(pkg *packageModel) []ComponentInfo {
	var components []ComponentInfo
	for _, file := range pkg.files {
		for _, decl := range file.Decls {
			generalDecl, ok := decl.(*ast.GenDecl)
			if !ok || generalDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range generalDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || structType.Fields == nil {
					continue
				}
				if kind, ok := scanComponentKind(pkg, structType); ok {
					pos := pkg.fset.Position(typeSpec.Pos())
					components = append(components, ComponentInfo{
						Package:  pkg.name,
						File:     pos.Filename,
						Line:     pos.Line,
						TypeName: typeSpec.Name.Name,
						Kind:     kind,
					})
				}
			}
		}
	}
	return components
}

func scanComponentKind(pkg *packageModel, structType *ast.StructType) (ComponentKind, bool) {
	for _, field := range structType.Fields.List {
		if len(field.Names) != 0 {
			continue
		}
		if kind, ok := componentKindFromExpr(pkg, field.Type); ok {
			return kind, true
		}
	}
	return "", false
}

func componentKindFromExpr(pkg *packageModel, expr ast.Expr) (ComponentKind, bool) {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	if obj := pkg.typesInfo.Uses[selector.Sel]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == helixPackagePath {
		return markerKind(selector.Sel.Name)
	}
	if ident, ok := selector.X.(*ast.Ident); ok && pkg.imports[ident.Name] == helixPackagePath {
		return markerKind(selector.Sel.Name)
	}
	return "", false
}

func markerKind(name string) (ComponentKind, bool) {
	switch name {
	case "Service":
		return ComponentService, true
	case "Controller":
		return ComponentController, true
	case "Repository":
		return ComponentRepository, true
	case "Component":
		return ComponentComponent, true
	default:
		return "", false
	}
}

func scanPackageDirectives(pkg *packageModel) ([]DirectiveInfo, error) {
	var directives []DirectiveInfo
	for _, file := range pkg.files {
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.FuncDecl:
				target := node.Name.Name
				if node.Recv != nil && len(node.Recv.List) > 0 {
					if receiver := receiverTypeName(node.Recv.List[0].Type); receiver != "" {
						target = receiver + "." + node.Name.Name
					}
				}
				found, err := parseHelixDirectiveComments(pkg, node.Doc, target)
				if err != nil {
					return nil, err
				}
				directives = append(directives, found...)
			case *ast.GenDecl:
				if node.Tok != token.TYPE {
					continue
				}
				for _, spec := range node.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
					if !ok || interfaceType.Methods == nil {
						continue
					}
					for _, field := range interfaceType.Methods.List {
						if len(field.Names) == 0 {
							continue
						}
						target := typeSpec.Name.Name + "." + field.Names[0].Name
						found, err := parseHelixDirectiveComments(pkg, field.Doc, target)
						if err != nil {
							return nil, err
						}
						directives = append(directives, found...)
					}
				}
			}
		}
	}
	return directives, nil
}

func parseHelixDirectiveComments(pkg *packageModel, comments *ast.CommentGroup, target string) ([]DirectiveInfo, error) {
	if comments == nil {
		return nil, nil
	}

	var directives []DirectiveInfo
	for _, comment := range comments.List {
		directive, ok, err := parseHelixDirectiveText(comment.Text)
		if err != nil {
			pos := pkg.fset.Position(comment.Pos())
			return nil, fmt.Errorf("cli/codegen: scan %s %s:%d %s: %w", pkg.name, pos.Filename, pos.Line, target, err)
		}
		if !ok {
			continue
		}
		pos := pkg.fset.Position(comment.Pos())
		directive.Package = pkg.name
		directive.File = pos.Filename
		directive.Line = pos.Line
		directive.Target = target
		directives = append(directives, directive)
	}
	return directives, nil
}

func parseHelixDirectiveText(text string) (DirectiveInfo, bool, error) {
	if isMalformedHelixDirective(text) {
		return DirectiveInfo{}, false, errInvalidHelixDirective
	}
	if !strings.HasPrefix(text, "//helix:") {
		return DirectiveInfo{}, false, nil
	}

	body := strings.TrimPrefix(text, "//helix:")
	name, argument := splitDirectiveBody(body)
	if name == "" {
		return DirectiveInfo{}, false, errInvalidHelixDirective
	}

	fields := strings.Fields(argument)
	normalizedArgument := strings.Join(fields, " ")
	switch name {
	case "route":
		if len(fields) != 2 || !isHTTPMethod(fields[0]) || !strings.HasPrefix(fields[1], "/") {
			return DirectiveInfo{}, false, errInvalidHelixDirective
		}
		normalizedArgument = strings.ToUpper(fields[0]) + " " + fields[1]
	case "guard", "interceptor":
		if len(fields) != 1 || !isNamedDirectiveArgument(fields[0]) {
			return DirectiveInfo{}, false, errInvalidHelixDirective
		}
	case "scheduled":
		if len(fields) == 0 {
			return DirectiveInfo{}, false, errInvalidHelixDirective
		}
	case "transactional":
		if len(fields) != 0 {
			return DirectiveInfo{}, false, errInvalidHelixDirective
		}
	case "query":
		if len(fields) != 1 || fields[0] != "auto" {
			return DirectiveInfo{}, false, errInvalidHelixDirective
		}
	default:
		return DirectiveInfo{}, false, errInvalidHelixDirective
	}

	return DirectiveInfo{Name: name, Argument: normalizedArgument, Raw: text}, true, nil
}

func isMalformedHelixDirective(text string) bool {
	return strings.HasPrefix(text, "// helix:") ||
		strings.HasPrefix(text, "//+helix:") ||
		strings.HasPrefix(text, "// +helix:")
}

func splitDirectiveBody(body string) (string, string) {
	for i, r := range body {
		if unicode.IsSpace(r) {
			return body[:i], strings.TrimSpace(body[i:])
		}
	}
	return body, ""
}

func isHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func isNamedDirectiveArgument(value string) bool {
	name, argument, hasArgument := strings.Cut(value, ":")
	if name == "" || !isDirectiveIdentifier(name) {
		return false
	}
	return !hasArgument || argument != ""
}

func isDirectiveIdentifier(value string) bool {
	if value == "" || value[len(value)-1] == '-' {
		return false
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		case i > 0 && r == '-':
		default:
			return false
		}
	}
	return true
}

func receiverTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func sourceOrder(leftFile string, leftLine int, leftName, rightFile string, rightLine int, rightName string) bool {
	if leftFile != rightFile {
		return leftFile < rightFile
	}
	if leftLine != rightLine {
		return leftLine < rightLine
	}
	return leftName < rightName
}
