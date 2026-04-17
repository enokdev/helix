package web

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

const controllerMarkerPkgPath = "github.com/enokdev/helix"

var (
	contextType = reflect.TypeOf((*Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

type routeConvention struct {
	method      string
	suffix      string
	handlerName string
}

type controllerRoute struct {
	method  string
	path    string
	handler HandlerFunc
	order   int
}

type routeDirective struct {
	method string
	path   string
}

var routeConventions = []routeConvention{
	{method: http.MethodGet, suffix: "", handlerName: "Index"},
	{method: http.MethodGet, suffix: "/:id", handlerName: "Show"},
	{method: http.MethodPost, suffix: "", handlerName: "Create"},
	{method: http.MethodPut, suffix: "/:id", handlerName: "Update"},
	{method: http.MethodDelete, suffix: "/:id", handlerName: "Delete"},
}

// RegisterController registers conventional REST routes for a marked
// controller. Routes are registered on the provided server before Start().
func RegisterController(server HTTPServer, controller any) error {
	if server == nil {
		return fmt.Errorf("web: register controller: %w", ErrInvalidController)
	}
	if sv := reflect.ValueOf(server); sv.Kind() == reflect.Ptr && sv.IsNil() {
		return fmt.Errorf("web: register controller: %w", ErrInvalidController)
	}

	controllerValue, controllerType, err := validateController(controller)
	if err != nil {
		return err
	}

	prefix, err := controllerRoutePrefix(controllerType.Name())
	if err != nil {
		return err
	}

	directives, err := controllerRouteDirectives(controllerValue.Type(), controllerType.Name())
	if err != nil {
		return err
	}

	// First pass: validate all handler signatures and route definitions before
	// registering any route, so a bad directive never leaves partial state.
	routes := make([]controllerRoute, 0, len(routeConventions)+len(directives))
	order := 0
	for _, convention := range routeConventions {
		method := controllerValue.MethodByName(convention.handlerName)
		if !method.IsValid() {
			continue
		}

		handler, err := adaptControllerMethod(method)
		if err != nil {
			return fmt.Errorf("web: register controller %s handler %s: %w", controllerType.Name(), convention.handlerName, err)
		}
		routes = append(routes, controllerRoute{
			method:  convention.method,
			path:    prefix + convention.suffix,
			handler: handler,
			order:   order,
		})
		order++
	}

	methodNames := make([]string, 0, len(directives))
	for methodName := range directives {
		methodNames = append(methodNames, methodName)
	}
	sort.Strings(methodNames)

	for _, methodName := range methodNames {
		methodDirectives := directives[methodName]
		method := controllerValue.MethodByName(methodName)
		if !method.IsValid() {
			continue
		}

		handler, err := adaptControllerMethod(method)
		if err != nil {
			return fmt.Errorf("web: register controller %s handler %s: %w", controllerType.Name(), methodName, err)
		}

		for _, directive := range methodDirectives {
			if _, err := validateRoute(directive.method, directive.path, handler); err != nil {
				return fmt.Errorf("web: register controller %s directive %s %s: %w", controllerType.Name(), directive.method, directive.path, ErrInvalidDirective)
			}
			routes = append(routes, controllerRoute{
				method:  directive.method,
				path:    directive.path,
				handler: handler,
				order:   order,
			})
			order++
		}
	}

	if len(routes) == 0 {
		return fmt.Errorf("web: register controller %s: %w", controllerType.Name(), ErrInvalidController)
	}

	sortControllerRoutes(routes)

	// Second pass: register all routes only after all handlers are validated.
	for _, r := range routes {
		if err := server.RegisterRoute(r.method, r.path, r.handler); err != nil {
			return fmt.Errorf("web: register controller %s route %s %s: %w", controllerType.Name(), r.method, r.path, err)
		}
	}
	return nil
}

func sortControllerRoutes(routes []controllerRoute) {
	sort.SliceStable(routes, func(i, j int) bool {
		iParameterized := strings.Contains(routes[i].path, ":")
		jParameterized := strings.Contains(routes[j].path, ":")
		if iParameterized != jParameterized {
			return !iParameterized
		}
		return routes[i].order < routes[j].order
	})
}

func controllerRouteDirectives(controllerMethodType reflect.Type, controllerName string) (map[string][]routeDirective, error) {
	files := make(map[string]*ast.File)
	fset := token.NewFileSet()
	directives := make(map[string][]routeDirective)

	for i := 0; i < controllerMethodType.NumMethod(); i++ {
		method := controllerMethodType.Method(i)
		runtimeFunc := runtime.FuncForPC(method.Func.Pointer())
		if runtimeFunc == nil {
			return nil, fmt.Errorf("web: parse directives for %s.%s: %w", controllerName, method.Name, ErrInvalidDirective)
		}

		filename, _ := runtimeFunc.FileLine(method.Func.Pointer())
		if filename == "" {
			return nil, fmt.Errorf("web: parse directives for %s.%s: %w", controllerName, method.Name, ErrInvalidDirective)
		}

		file, ok := files[filename]
		if !ok {
			parsed, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
			if err != nil {
				return nil, fmt.Errorf("web: parse directives in %s: %w", filename, ErrInvalidDirective)
			}
			file = parsed
			files[filename] = file
		}

		methodDirectives, err := parseMethodRouteDirectives(file, controllerName, method.Name)
		if err != nil {
			return nil, err
		}
		if len(methodDirectives) > 0 {
			directives[method.Name] = methodDirectives
		}
	}

	return directives, nil
}

func parseMethodRouteDirectives(file *ast.File, controllerName, methodName string) ([]routeDirective, error) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Name.Name != methodName {
			continue
		}
		if !receiverMatches(funcDecl.Recv, controllerName) {
			continue
		}

		return parseRouteDirectiveComments(controllerName, methodName, funcDecl.Doc)
	}
	return nil, nil
}

func receiverMatches(recv *ast.FieldList, controllerName string) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}

	receiverType := recv.List[0].Type
	if star, ok := receiverType.(*ast.StarExpr); ok {
		receiverType = star.X
	}

	ident, ok := receiverType.(*ast.Ident)
	return ok && ident.Name == controllerName
}

func parseRouteDirectiveComments(controllerName, methodName string, comments *ast.CommentGroup) ([]routeDirective, error) {
	if comments == nil {
		return nil, nil
	}

	directives := make([]routeDirective, 0, len(comments.List))
	for _, comment := range comments.List {
		text := comment.Text
		switch {
		case strings.HasPrefix(text, "//helix:route "):
			directive, err := parseRouteDirective(text)
			if err != nil {
				return nil, fmt.Errorf("web: parse directive %s.%s %q: %w", controllerName, methodName, text, err)
			}
			directives = append(directives, directive)
		case strings.HasPrefix(text, "// helix:route") || strings.HasPrefix(text, "//+helix:route") || strings.HasPrefix(text, "// +helix:route"):
			return nil, fmt.Errorf("web: parse directive %s.%s %q: %w", controllerName, methodName, text, ErrInvalidDirective)
		case strings.HasPrefix(text, "//helix:route") && !strings.HasPrefix(text, "//helix:route "):
			return nil, fmt.Errorf("web: parse directive %s.%s %q: %w", controllerName, methodName, text, ErrInvalidDirective)
		}
	}
	return directives, nil
}

func parseRouteDirective(text string) (routeDirective, error) {
	fields := strings.Fields(strings.TrimPrefix(text, "//helix:route "))
	if len(fields) != 2 {
		return routeDirective{}, ErrInvalidDirective
	}

	directive := routeDirective{
		method: fields[0],
		path:   fields[1],
	}
	normalizedMethod, err := validateRoute(directive.method, directive.path, func(Context) error { return nil })
	if err != nil {
		return routeDirective{}, ErrInvalidDirective
	}
	directive.method = normalizedMethod
	return directive, nil
}

func validateController(controller any) (reflect.Value, reflect.Type, error) {
	value := reflect.ValueOf(controller)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return reflect.Value{}, nil, fmt.Errorf("web: validate controller %T: %w", controller, ErrInvalidController)
	}

	controllerType := value.Type().Elem()
	if controllerType.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("web: validate controller %T: %w", controller, ErrInvalidController)
	}
	if !strings.HasSuffix(controllerType.Name(), "Controller") {
		return reflect.Value{}, nil, fmt.Errorf("web: validate controller %s name: %w", controllerType.Name(), ErrInvalidController)
	}
	if !hasControllerMarker(controllerType) {
		return reflect.Value{}, nil, fmt.Errorf("web: validate controller %s marker: %w", controllerType.Name(), ErrInvalidController)
	}

	return value, controllerType, nil
}

func hasControllerMarker(controllerType reflect.Type) bool {
	for i := 0; i < controllerType.NumField(); i++ {
		field := controllerType.Field(i)
		if !field.Anonymous {
			continue
		}
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() != reflect.Struct {
			continue
		}
		if fieldType.Name() == "Controller" && fieldType.PkgPath() == controllerMarkerPkgPath {
			return true
		}
	}
	return false
}

func controllerRoutePrefix(controllerName string) (string, error) {
	baseName := strings.TrimSuffix(controllerName, "Controller")
	if baseName == "" {
		return "", fmt.Errorf("web: derive route prefix %s: %w", controllerName, ErrInvalidController)
	}

	segments := pascalWords(baseName)
	if len(segments) == 0 {
		return "", fmt.Errorf("web: derive route prefix %s: %w", controllerName, ErrInvalidController)
	}
	segments[len(segments)-1] = pluralize(segments[len(segments)-1])

	return "/" + strings.Join(segments, "-"), nil
}

func pascalWords(value string) []string {
	runes := []rune(value)
	words := make([]string, 0, len(runes))
	start := 0

	for i := 1; i < len(runes); i++ {
		current := runes[i]
		previous := runes[i-1]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		if unicode.IsUpper(current) && (unicode.IsLower(previous) || unicode.IsDigit(previous) || (next != 0 && unicode.IsLower(next))) {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}

	words = append(words, strings.ToLower(string(runes[start:])))
	return words
}

func pluralize(word string) string {
	if strings.HasSuffix(word, "y") && !hasVowelSuffix(word, 2) {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	if strings.HasSuffix(word, "s") {
		return word + "es"
	}
	return word + "s"
}

func hasVowelSuffix(word string, offsetFromEnd int) bool {
	if len(word) < offsetFromEnd {
		return false
	}
	switch word[len(word)-offsetFromEnd] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

func adaptControllerMethod(method reflect.Value) (HandlerFunc, error) {
	methodType := method.Type()
	if methodType.NumIn() > 1 || methodType.NumOut() > 1 {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}
	if methodType.NumIn() == 1 && methodType.In(0) != contextType {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}
	if methodType.NumOut() == 1 && !methodType.Out(0).Implements(errorType) {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}

	return func(ctx Context) error {
		if ctx == nil {
			return fmt.Errorf("web: adapt handler: nil context")
		}

		args := []reflect.Value(nil)
		if methodType.NumIn() == 1 {
			args = []reflect.Value{reflect.ValueOf(ctx)}
		}

		results := method.Call(args)
		if len(results) == 0 || isNilReflectValue(results[0]) {
			return nil
		}

		// Safe: Out(0).Implements(errorType) was verified above.
		return results[0].Interface().(error) //nolint:forcetypeassert
	}, nil
}

func isNilReflectValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
