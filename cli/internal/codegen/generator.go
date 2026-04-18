package codegen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

var (
	errInvalidDirective = errors.New("invalid query directive")
	errInvalidQuery     = errors.New("invalid query method")
	errInvalidPackage   = errors.New("invalid package")
)

// Result describes the outcome of one generation run.
type Result struct {
	GeneratedFiles int
}

// Generator creates repository query implementations for one directory tree.
type Generator struct {
	dir string
}

// NewGenerator creates a repository query generator rooted at dir.
func NewGenerator(dir string) *Generator {
	return &Generator{dir: dir}
}

// Generate scans the generator root and writes deterministic *_gen.go files.
func (g *Generator) Generate(ctx context.Context) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("cli/codegen: generate: nil context: %w", errInvalidPackage)
	}
	root := g.dir
	if root == "" {
		root = "."
	}

	var generated int
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
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
		count, err := generatePackage(pkg)
		if err != nil {
			return err
		}
		generated += count
		return nil
	}); err != nil {
		return Result{}, fmt.Errorf("cli/codegen: generate %s: %w", root, err)
	}

	return Result{GeneratedFiles: generated}, nil
}

type packageModel struct {
	dir           string
	name          string
	fset          *token.FileSet
	files         []*ast.File
	structs       map[string]map[string]struct{}
	existingFuncs map[string]struct{}
	embeds        map[string][]string // structName → embedded local type names
}

type repositoryModel struct {
	PackageName string
	Interface   string
	Entity      string
	ID          string
	Methods     []queryMethod
}

type queryMethod struct {
	Name       string
	Return     returnKind
	Predicates []queryPredicate
	Order      *queryOrder
}

type queryPredicate struct {
	Field    string
	Column   string
	Param    string
	Type     string
	Operator queryOperator
}

type queryOrder struct {
	Field  string
	Column string
	Desc   bool
}

type queryOperator string

const (
	operatorEqual       queryOperator = "equal"
	operatorContains    queryOperator = "contains"
	operatorGreaterThan queryOperator = "greaterThan"
)

type returnKind string

const (
	returnOne  returnKind = "one"
	returnMany returnKind = "many"
)

func shouldSkipDir(name string) bool {
	return name == "vendor" || strings.HasPrefix(name, ".") || name == "testdata"
}

func loadPackage(dir string) (*packageModel, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !isSourceFile(entry.Name()) {
			continue
		}
		match, err := build.Default.MatchFile(dir, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("match build constraints %s: %w", filepath.Join(dir, entry.Name()), err)
		}
		if !match {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, nil
	}

	sort.Slice(files, func(i, j int) bool {
		return fset.Position(files[i].Package).Filename < fset.Position(files[j].Package).Filename
	})

	pkg := &packageModel{
		dir:           dir,
		name:          files[0].Name.Name,
		fset:          fset,
		files:         files,
		structs:       make(map[string]map[string]struct{}),
		existingFuncs: make(map[string]struct{}),
		embeds:        make(map[string][]string),
	}
	for _, file := range files {
		if file.Name.Name != pkg.name {
			return nil, fmt.Errorf("package name mismatch in %s: %w", fset.Position(file.Package).Filename, errInvalidPackage)
		}
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				pkg.collectTypes(node)
			case *ast.FuncDecl:
				pkg.existingFuncs[node.Name.Name] = struct{}{}
			}
		}
	}
	pkg.resolveEmbeds()
	return pkg, nil
}

func isSourceFile(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, "_gen.go")
}

func (p *packageModel) collectTypes(decl *ast.GenDecl) {
	if decl.Tok != token.TYPE {
		return
	}
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok || structType.Fields == nil {
			continue
		}
		fields := make(map[string]struct{})
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				// Anonymous embedded field — record for resolution after all types are collected.
				if ident, ok := field.Type.(*ast.Ident); ok {
					p.embeds[typeSpec.Name.Name] = append(p.embeds[typeSpec.Name.Name], ident.Name)
				}
				continue
			}
			for _, name := range field.Names {
				fields[name.Name] = struct{}{}
			}
		}
		p.structs[typeSpec.Name.Name] = fields
	}
}

// resolveEmbeds merges promoted fields from anonymous embedded types into their parent struct.
// Iterates until stable to handle chains of embedding.
func (p *packageModel) resolveEmbeds() {
	for changed := true; changed; {
		changed = false
		for structName, embedded := range p.embeds {
			for _, embedName := range embedded {
				embedFields, ok := p.structs[embedName]
				if !ok {
					continue
				}
				for field := range embedFields {
					if _, exists := p.structs[structName][field]; !exists {
						p.structs[structName][field] = struct{}{}
						changed = true
					}
				}
			}
		}
	}
}

func generatePackage(pkg *packageModel) (int, error) {
	repositories, err := discoverRepositories(pkg)
	if err != nil {
		return 0, err
	}
	for _, repository := range repositories {
		content, err := renderRepository(repository)
		if err != nil {
			return 0, err
		}
		formatted, err := format.Source(content)
		if err != nil {
			return 0, fmt.Errorf("format generated repository %s: %w", repository.Interface, err)
		}
		path := filepath.Join(pkg.dir, snakeName(repository.Interface)+"_query_gen.go")
		if err := writeFileIfChanged(path, formatted); err != nil {
			return 0, err
		}
	}
	return len(repositories), nil
}

func discoverRepositories(pkg *packageModel) ([]repositoryModel, error) {
	var repositories []repositoryModel
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
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok || interfaceType.Methods == nil {
					continue
				}
				repository, ok, err := parseRepositoryInterface(pkg, typeSpec.Name.Name, interfaceType)
				if err != nil {
					return nil, err
				}
				if ok {
					repositories = append(repositories, repository)
				}
			}
		}
	}

	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Interface < repositories[j].Interface
	})
	return repositories, nil
}

func parseRepositoryInterface(pkg *packageModel, name string, interfaceType *ast.InterfaceType) (repositoryModel, bool, error) {
	repository := repositoryModel{PackageName: pkg.name, Interface: name}
	for _, field := range interfaceType.Methods.List {
		if len(field.Names) != 0 {
			continue
		}
		entity, id, ok := parseRepositoryEmbed(field.Type)
		if ok {
			repository.Entity = entity
			repository.ID = id
			break
		}
	}
	if repository.Entity == "" {
		return repositoryModel{}, false, nil
	}
	if _, ok := pkg.structs[repository.Entity]; !ok {
		return repositoryModel{}, false, fmt.Errorf("cli/codegen: repository %s entity %s not found: %w", name, repository.Entity, errInvalidQuery)
	}

	constructor := "New" + name
	if _, exists := pkg.existingFuncs[constructor]; exists {
		return repositoryModel{}, false, fmt.Errorf("cli/codegen: repository %s constructor %s already exists: %w", name, constructor, errInvalidQuery)
	}

	for _, field := range interfaceType.Methods.List {
		if len(field.Names) == 0 {
			continue
		}
		methodName := field.Names[0].Name
		hasDirective, err := parseQueryDirective(name, methodName, field.Doc)
		if err != nil {
			return repositoryModel{}, false, err
		}
		if !hasDirective {
			continue
		}
		funcType, ok := field.Type.(*ast.FuncType)
		if !ok {
			return repositoryModel{}, false, fmt.Errorf("cli/codegen: repository %s method %s is not a function: %w", name, methodName, errInvalidQuery)
		}
		method, err := parseQueryMethod(pkg, repository, methodName, funcType)
		if err != nil {
			return repositoryModel{}, false, fmt.Errorf("cli/codegen: parse query method %s.%s: %w", name, methodName, err)
		}
		repository.Methods = append(repository.Methods, method)
	}
	if len(repository.Methods) == 0 {
		return repositoryModel{}, false, nil
	}
	return repository, true, nil
}

func parseRepositoryEmbed(expr ast.Expr) (string, string, bool) {
	index, ok := expr.(*ast.IndexListExpr)
	if !ok || len(index.Indices) != 3 {
		return "", "", false
	}
	if !isRepositoryExpr(index.X) {
		return "", "", false
	}
	entity := exprString(index.Indices[0])
	id := exprString(index.Indices[1])
	return entity, id, true
}

func isRepositoryExpr(expr ast.Expr) bool {
	switch node := expr.(type) {
	case *ast.SelectorExpr:
		return node.Sel.Name == "Repository"
	case *ast.Ident:
		return node.Name == "Repository"
	default:
		return false
	}
}

func parseQueryDirective(iface, method string, comments *ast.CommentGroup) (bool, error) {
	if comments == nil {
		return false, nil
	}

	found := false
	for _, comment := range comments.List {
		text := comment.Text
		switch {
		case text == "//helix:query auto":
			if found {
				return false, fmt.Errorf("cli/codegen: parse query directive %s.%s: %w", iface, method, errInvalidDirective)
			}
			found = true
		case strings.HasPrefix(text, "// helix:query") ||
			strings.HasPrefix(text, "//+helix:query") ||
			strings.HasPrefix(text, "// +helix:query") ||
			strings.HasPrefix(text, "//helix:query"):
			return false, fmt.Errorf("cli/codegen: parse query directive %s.%s: %w", iface, method, errInvalidDirective)
		}
	}
	return found, nil
}

func parseQueryMethod(pkg *packageModel, repository repositoryModel, name string, funcType *ast.FuncType) (queryMethod, error) {
	if funcType.Params == nil || len(funcType.Params.List) == 0 || !isContextType(funcType.Params.List[0].Type) {
		return queryMethod{}, fmt.Errorf("missing context.Context first parameter: %w", errInvalidQuery)
	}
	returnKind, err := parseReturns(repository.Entity, funcType.Results)
	if err != nil {
		return queryMethod{}, err
	}

	method := queryMethod{Name: name, Return: returnKind}
	switch {
	case strings.HasPrefix(name, "FindAllOrderBy"):
		order, err := parseOrderMethod(pkg, repository, name)
		if err != nil {
			return queryMethod{}, err
		}
		if returnKind != returnMany {
			return queryMethod{}, fmt.Errorf("FindAllOrderBy must return []%s: %w", repository.Entity, errInvalidQuery)
		}
		if countParams(funcType.Params) != 1 {
			return queryMethod{}, fmt.Errorf("FindAllOrderBy takes only context.Context: %w", errInvalidQuery)
		}
		method.Order = &order
	case strings.HasPrefix(name, "FindBy"):
		predicates, err := parseFindByMethod(pkg, repository, name, funcType)
		if err != nil {
			return queryMethod{}, err
		}
		method.Predicates = predicates
	default:
		return queryMethod{}, fmt.Errorf("unsupported method name %s: %w", name, errInvalidQuery)
	}
	return method, nil
}

func parseReturns(entity string, results *ast.FieldList) (returnKind, error) {
	if results == nil || len(results.List) != 2 {
		return "", fmt.Errorf("query methods must return payload and error: %w", errInvalidQuery)
	}
	if !isErrorType(results.List[1].Type) {
		return "", fmt.Errorf("query methods must return error as second result: %w", errInvalidQuery)
	}

	first := results.List[0].Type
	if ptr, ok := first.(*ast.StarExpr); ok && exprString(ptr.X) == entity {
		return returnOne, nil
	}
	if array, ok := first.(*ast.ArrayType); ok && array.Len == nil && exprString(array.Elt) == entity {
		return returnMany, nil
	}
	return "", fmt.Errorf("unsupported query return %s: %w", exprString(first), errInvalidQuery)
}

func parseFindByMethod(pkg *packageModel, repository repositoryModel, name string, funcType *ast.FuncType) ([]queryPredicate, error) {
	raw := strings.TrimPrefix(name, "FindBy")
	if raw == "" {
		return nil, fmt.Errorf("missing predicate: %w", errInvalidQuery)
	}
	parts := splitOnAnd(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("missing predicate: %w", errInvalidQuery)
	}

	paramNames, paramTypes := methodParams(funcType.Params)
	if len(paramNames) != len(parts) {
		return nil, fmt.Errorf("predicate count %d does not match parameter count %d: %w", len(parts), len(paramNames), errInvalidQuery)
	}

	predicates := make([]queryPredicate, 0, len(parts))
	for i, part := range parts {
		field, operator := splitPredicate(part)
		if field == "" {
			return nil, fmt.Errorf("empty predicate in %s: %w", name, errInvalidQuery)
		}
		if _, ok := pkg.structs[repository.Entity][field]; !ok {
			return nil, fmt.Errorf("field %s not found on %s: %w", field, repository.Entity, errInvalidQuery)
		}
		if operator == operatorContains && exprString(paramTypes[i]) != "string" {
			return nil, fmt.Errorf("Containing predicate %s requires string parameter: %w", field, errInvalidQuery)
		}
		predicates = append(predicates, queryPredicate{
			Field:    field,
			Column:   lowerFirst(field) + "Column",
			Param:    paramNames[i],
			Type:     exprString(paramTypes[i]),
			Operator: operator,
		})
	}
	return predicates, nil
}

func splitPredicate(part string) (string, queryOperator) {
	for _, candidate := range []struct {
		suffix   string
		operator queryOperator
	}{
		{suffix: "Containing", operator: operatorContains},
		{suffix: "GreaterThan", operator: operatorGreaterThan},
	} {
		if strings.HasSuffix(part, candidate.suffix) {
			return strings.TrimSuffix(part, candidate.suffix), candidate.operator
		}
	}
	return part, operatorEqual
}

func parseOrderMethod(pkg *packageModel, repository repositoryModel, name string) (queryOrder, error) {
	raw := strings.TrimPrefix(name, "FindAllOrderBy")
	var desc bool
	switch {
	case strings.HasSuffix(raw, "Desc"):
		desc = true
		raw = strings.TrimSuffix(raw, "Desc")
	case strings.HasSuffix(raw, "Asc"):
		raw = strings.TrimSuffix(raw, "Asc")
	default:
		return queryOrder{}, fmt.Errorf("order method must end with Asc or Desc: %w", errInvalidQuery)
	}
	if raw == "" {
		return queryOrder{}, fmt.Errorf("order field missing: %w", errInvalidQuery)
	}
	if _, ok := pkg.structs[repository.Entity][raw]; !ok {
		return queryOrder{}, fmt.Errorf("field %s not found on %s: %w", raw, repository.Entity, errInvalidQuery)
	}
	return queryOrder{Field: raw, Column: lowerFirst(raw) + "Column", Desc: desc}, nil
}

func methodParams(params *ast.FieldList) ([]string, []ast.Expr) {
	var names []string
	var types []ast.Expr
	for i, field := range params.List {
		if i == 0 {
			continue
		}
		if len(field.Names) == 0 {
			names = append(names, fmt.Sprintf("arg%d", len(names)+1))
			types = append(types, field.Type)
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
			types = append(types, field.Type)
		}
	}
	return names, types
}

func countParams(params *ast.FieldList) int {
	if params == nil {
		return 0
	}
	count := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}

func isContextType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "context"
}

func isErrorType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func renderRepository(repository repositoryModel) ([]byte, error) {
	var buffer bytes.Buffer
	structName := lowerFirst(repository.Interface) + "Gen"

	fmt.Fprintf(&buffer, "// Code generated by helix generate. DO NOT EDIT.\n\n")
	fmt.Fprintf(&buffer, "package %s\n\n", repository.PackageName)
	fmt.Fprintf(&buffer, "import (\n")
	fmt.Fprintf(&buffer, "\t\"context\"\n\n")
	fmt.Fprintf(&buffer, "\tdatagorm \"github.com/enokdev/helix/data/gorm\"\n")
	fmt.Fprintf(&buffer, "\tgormlib \"gorm.io/gorm\"\n")
	fmt.Fprintf(&buffer, "\t\"gorm.io/gorm/clause\"\n")
	fmt.Fprintf(&buffer, ")\n\n")

	fmt.Fprintf(&buffer, "type %s struct {\n", structName)
	fmt.Fprintf(&buffer, "\t*datagorm.Repository[%s, %s]\n", repository.Entity, repository.ID)
	fmt.Fprintf(&buffer, "\tdb *gormlib.DB\n")
	fmt.Fprintf(&buffer, "}\n\n")

	fmt.Fprintf(&buffer, "func New%s(db *gormlib.DB) %s {\n", repository.Interface, repository.Interface)
	fmt.Fprintf(&buffer, "\tbase := datagorm.NewRepository[%s, %s](db)\n", repository.Entity, repository.ID)
	fmt.Fprintf(&buffer, "\treturn &%s{Repository: base, db: db}\n", structName)
	fmt.Fprintf(&buffer, "}\n\n")

	for _, method := range repository.Methods {
		renderMethod(&buffer, repository, structName, method)
	}
	return buffer.Bytes(), nil
}

func renderMethod(buffer *bytes.Buffer, repository repositoryModel, structName string, method queryMethod) {
	switch method.Return {
	case returnOne:
		fmt.Fprintf(buffer, "func (r *%s) %s(ctx context.Context", structName, method.Name)
		renderMethodParams(buffer, method)
		fmt.Fprintf(buffer, ") (*%s, error) {\n", repository.Entity)
	case returnMany:
		fmt.Fprintf(buffer, "func (r *%s) %s(ctx context.Context", structName, method.Name)
		renderMethodParams(buffer, method)
		fmt.Fprintf(buffer, ") ([]%s, error) {\n", repository.Entity)
	}

	action := "query " + method.Name
	fmt.Fprintf(buffer, "\tdb, err := datagorm.Database(ctx, r.db, %q)\n", action)
	fmt.Fprintf(buffer, "\tif err != nil {\n")
	renderReturnError(buffer, method.Return, repository.Entity, "err")
	fmt.Fprintf(buffer, "\t}\n\n")

	renderColumnLookups(buffer, repository, method, action)

	if method.Order != nil {
		renderOrderQuery(buffer, repository, method, action)
	} else {
		renderPredicateQuery(buffer, repository, method, action)
	}
	fmt.Fprintf(buffer, "}\n\n")
}

func renderMethodParams(buffer *bytes.Buffer, method queryMethod) {
	for _, predicate := range method.Predicates {
		fmt.Fprintf(buffer, ", %s %s", predicate.Param, predicate.Type)
	}
}

func renderColumnLookups(buffer *bytes.Buffer, repository repositoryModel, method queryMethod, action string) {
	for _, predicate := range method.Predicates {
		fmt.Fprintf(buffer, "\t%s, err := datagorm.ColumnFor[%s](db, %q)\n", predicate.Column, repository.Entity, predicate.Field)
		fmt.Fprintf(buffer, "\tif err != nil {\n")
		renderReturnError(buffer, method.Return, repository.Entity, fmt.Sprintf("datagorm.WrapError(%q, err)", action))
		fmt.Fprintf(buffer, "\t}\n")
	}
	if method.Order != nil {
		fmt.Fprintf(buffer, "\t%s, err := datagorm.ColumnFor[%s](db, %q)\n", method.Order.Column, repository.Entity, method.Order.Field)
		fmt.Fprintf(buffer, "\tif err != nil {\n")
		renderReturnError(buffer, method.Return, repository.Entity, fmt.Sprintf("datagorm.WrapError(%q, err)", action))
		fmt.Fprintf(buffer, "\t}\n")
	}
	if len(method.Predicates) > 0 || method.Order != nil {
		fmt.Fprintln(buffer)
	}
}

func renderPredicateQuery(buffer *bytes.Buffer, repository repositoryModel, method queryMethod, action string) {
	fmt.Fprintf(buffer, "\tquery := db")
	for _, predicate := range method.Predicates {
		switch predicate.Operator {
		case operatorEqual:
			fmt.Fprintf(buffer, "\n\tquery = query.Where(clause.Eq{Column: %s, Value: %s})", predicate.Column, predicate.Param)
		case operatorGreaterThan:
			fmt.Fprintf(buffer, "\n\tquery = query.Where(clause.Gt{Column: %s, Value: %s})", predicate.Column, predicate.Param)
		case operatorContains:
			fmt.Fprintf(buffer, "\n\tquery = query.Where(clause.Expr{SQL: \"? LIKE ? ESCAPE '\\\\'\", Vars: []any{%s, \"%%\" + datagorm.EscapeLike(%s) + \"%%\"}})", predicate.Column, predicate.Param)
		}
	}
	fmt.Fprintln(buffer)
	renderExecuteQuery(buffer, repository, method.Return, action)
}

func renderOrderQuery(buffer *bytes.Buffer, repository repositoryModel, method queryMethod, action string) {
	fmt.Fprintf(buffer, "\tquery := db.Order(clause.OrderByColumn{Column: %s, Desc: %t})\n", method.Order.Column, method.Order.Desc)
	renderExecuteQuery(buffer, repository, method.Return, action)
}

func renderExecuteQuery(buffer *bytes.Buffer, repository repositoryModel, returnKind returnKind, action string) {
	switch returnKind {
	case returnOne:
		fmt.Fprintf(buffer, "\tvar item %s\n", repository.Entity)
		fmt.Fprintf(buffer, "\tif err := query.Take(&item).Error; err != nil {\n")
		fmt.Fprintf(buffer, "\t\treturn nil, datagorm.WrapError(%q, err)\n", action)
		fmt.Fprintf(buffer, "\t}\n")
		fmt.Fprintf(buffer, "\treturn &item, nil\n")
	case returnMany:
		fmt.Fprintf(buffer, "\tvar items []%s\n", repository.Entity)
		fmt.Fprintf(buffer, "\tif err := query.Find(&items).Error; err != nil {\n")
		fmt.Fprintf(buffer, "\t\treturn nil, datagorm.WrapError(%q, err)\n", action)
		fmt.Fprintf(buffer, "\t}\n")
		fmt.Fprintf(buffer, "\treturn items, nil\n")
	}
}

func renderReturnError(buffer *bytes.Buffer, returnKind returnKind, entity, errExpr string) {
	switch returnKind {
	case returnOne:
		fmt.Fprintf(buffer, "\t\treturn nil, %s\n", errExpr)
	case returnMany:
		fmt.Fprintf(buffer, "\t\treturn nil, %s\n", errExpr)
	default:
		fmt.Fprintf(buffer, "\t\treturn nil, %s\n", errExpr)
	}
}

func writeFileIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing generated file %s: %w", path, err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp generated file %s: %w", path, err)
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp generated file %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp generated file %s: %w", path, err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace generated file %s: %w", path, err)
	}
	return nil
}

func exprString(expr ast.Expr) string {
	var buffer bytes.Buffer
	_ = printer.Fprint(&buffer, token.NewFileSet(), expr)
	return buffer.String()
}

// splitOnAnd splits s on "And" only when followed by an uppercase letter,
// avoiding false splits on field names that contain "and" as a substring (e.g. "BandName").
func splitOnAnd(s string) []string {
	var parts []string
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "And") && i+3 < len(s) && unicode.IsUpper(rune(s[i+3])) {
			parts = append(parts, s[:i])
			s = s[i+3:]
			i = 0
		} else {
			i++
		}
	}
	return append(parts, s)
}

func snakeName(value string) string {
	runes := []rune(value)
	var builder strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			// Insert underscore on lower→upper transition, or at the end of an
			// uppercase run before the next lowercase letter (e.g. "URLPath" → "url_path").
			if unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				builder.WriteByte('_')
			}
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

func lowerFirst(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}
