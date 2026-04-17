package helix

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type scanResult struct {
	ComponentCount int
}

func scanComponentMarkers(patterns []string) (scanResult, error) {
	var result scanResult
	for _, pattern := range patterns {
		count, err := scanComponentPattern(pattern)
		if err != nil {
			return scanResult{}, err
		}
		result.ComponentCount += count
	}
	return result, nil
}

func scanComponentPattern(pattern string) (int, error) {
	root, recursive := scanRoot(pattern)
	info, err := os.Stat(root)
	if err != nil {
		return 0, fmt.Errorf("helix: scan %q: %w", pattern, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("helix: scan %q: not a directory: %w", pattern, ErrInvalidComponent)
	}

	count := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (!recursive || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isScannableGoFile(entry.Name()) {
			return nil
		}

		fileCount, err := scanGoFileForMarkers(path)
		if err != nil {
			return err
		}
		count += fileCount
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("helix: scan %q: %w", pattern, err)
	}
	return count, nil
}

func scanRoot(pattern string) (string, bool) {
	const recursiveSuffix = string(filepath.Separator) + "..."
	if strings.HasSuffix(pattern, recursiveSuffix) {
		return strings.TrimSuffix(pattern, recursiveSuffix), true
	}
	if pattern == "..." {
		return ".", true
	}
	return pattern, false
}

func isScannableGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, "_gen.go")
}

func scanGoFileForMarkers(path string) (int, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return 0, err
	}

	count := 0
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
			if structHasMarkerEmbed(structType) {
				count++
			}
		}
	}
	return count, nil
}

func structHasMarkerEmbed(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if len(field.Names) != 0 {
			continue
		}
		if astTypeIsMarker(field.Type) {
			return true
		}
	}
	return false
}

// astTypeIsMarker reports whether expr is a qualified helix marker embed
// (e.g. helix.Service). Pointer embeds and unqualified names are not matched.
func astTypeIsMarker(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "helix" && isMarkerName(sel.Sel.Name)
}

func isMarkerName(name string) bool {
	switch name {
	case "Service", "Controller", "Repository", "Component":
		return true
	default:
		return false
	}
}
