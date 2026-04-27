package web

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/enokdev/helix/web/internal"
)

const errorHandlerMarkerName = "ErrorHandler"

var intType = reflect.TypeOf(0)

type errorHandlerRegistrar interface {
	registerErrorHandler(handler any) error
}

type errorHandlerInvoker func(Context, error) (bool, error)

type handlesDirective struct {
	errorType string
}

// RegisterErrorHandler registers centralized error mappings on a Helix server.
func RegisterErrorHandler(server HTTPServer, handler any) error {
	registrar, ok := server.(errorHandlerRegistrar)
	if !ok || registrar == nil {
		return fmt.Errorf("web: register error handler: %w", ErrInvalidErrorHandler)
	}
	if err := registrar.registerErrorHandler(handler); err != nil {
		return fmt.Errorf("web: register error handler: %w", err)
	}
	return nil
}

// tryGetGeneratedErrorHandlers checks the registry for pre-generated error handlers.
// If handlers are found in the registry, they are converted to the directives format
// and returned. Otherwise, returns nil, false to signal AST parsing fallback.
func tryGetGeneratedErrorHandlers(handlerName string) (map[string]handlesDirective, bool) {
	registry := internal.GlobalErrorHandlerRegistry()
	if !registry.HasGeneratedErrorHandlers() {
		return nil, false
	}

	handlers, ok := registry.GetErrorHandlersForHandler(handlerName)
	if !ok || len(handlers) == 0 {
		return nil, false
	}

	// Convert registry ErrorHandlerInfo entries into handlesDirective format
	directives := make(map[string]handlesDirective)
	for i, handler := range handlers {
		// Each handler entry maps to a method name
		// Use a generated method name if not provided
		methodName := fmt.Sprintf("Handle%d", i)
		if handler.MethodName != "" {
			methodName = handler.MethodName
		}

		directives[methodName] = handlesDirective{
			errorType: handler.ErrorType,
		}
	}

	slog.Debug("using generated error handlers from registry", "handler", handlerName, "count", len(handlers))
	return directives, true
}

func buildErrorHandlers(server HTTPServer, handler any) (map[string]errorHandlerInvoker, error) {
	handlerValue, handlerType, err := validateErrorHandler(handler)
	if err != nil {
		return nil, err
	}

	// First, try to get directives from the generated registry
	directives, hasGenerated := tryGetGeneratedErrorHandlers(handlerType.Name())
	
	// Check if server enforces generated only mode
	generatedOnly := false
	if srv, ok := server.(interface{ IsGeneratedOnly() bool }); ok {
		generatedOnly = srv.IsGeneratedOnly()
	}

	if !hasGenerated {
		if generatedOnly {
			return nil, fmt.Errorf("web: build error handler %s: generated registry empty and GeneratedOnly mode enabled", handlerType.Name())
		}

		// Fall back to AST parsing if no generated handlers are found
		var err error
		directives, err = errorHandlerDirectives(handlerValue.Type(), handlerType.Name())
		if err != nil {
			return nil, err
		}
		if len(directives) > 0 {
			slog.Debug("using AST-parsed error handlers (no generated registry found)", "handler", handlerType.Name())
		}
	}
	if len(directives) == 0 {
		return nil, fmt.Errorf("web: validate error handler %s directives: %w", handlerType.Name(), ErrInvalidErrorHandler)
	}

	methodNames := make([]string, 0, len(directives))
	for methodName := range directives {
		methodNames = append(methodNames, methodName)
	}
	sort.Strings(methodNames)

	handlers := make(map[string]errorHandlerInvoker, len(methodNames))
	for _, methodName := range methodNames {
		method := handlerValue.MethodByName(methodName)
		if !method.IsValid() {
			continue
		}
		invoker, err := newErrorHandlerInvoker(method, directives[methodName].errorType)
		if err != nil {
			return nil, fmt.Errorf("web: validate error handler %s.%s: %w", handlerType.Name(), methodName, err)
		}
		if _, exists := handlers[directives[methodName].errorType]; exists {
			return nil, fmt.Errorf("web: validate error handler %s.%s duplicate %s: %w", handlerType.Name(), methodName, directives[methodName].errorType, ErrInvalidErrorHandler)
		}
		handlers[directives[methodName].errorType] = invoker
	}
	if len(handlers) == 0 {
		return nil, fmt.Errorf("web: validate error handler %s directives: %w", handlerType.Name(), ErrInvalidErrorHandler)
	}
	return handlers, nil
}

func validateErrorHandler(handler any) (reflect.Value, reflect.Type, error) {
	value := reflect.ValueOf(handler)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return reflect.Value{}, nil, fmt.Errorf("web: validate error handler %T: %w", handler, ErrInvalidErrorHandler)
	}

	handlerType := value.Type().Elem()
	if handlerType.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("web: validate error handler %T: %w", handler, ErrInvalidErrorHandler)
	}
	if !strings.HasSuffix(handlerType.Name(), errorHandlerMarkerName) {
		return reflect.Value{}, nil, fmt.Errorf("web: validate error handler %s name: %w", handlerType.Name(), ErrInvalidErrorHandler)
	}
	if !hasErrorHandlerMarker(handlerType) {
		return reflect.Value{}, nil, fmt.Errorf("web: validate error handler %s marker: %w", handlerType.Name(), ErrInvalidErrorHandler)
	}

	return value, handlerType, nil
}

func hasErrorHandlerMarker(handlerType reflect.Type) bool {
	for i := 0; i < handlerType.NumField(); i++ {
		field := handlerType.Field(i)
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
		if fieldType.Name() == errorHandlerMarkerName && fieldType.PkgPath() == controllerMarkerPkgPath {
			return true
		}
	}
	return false
}

func errorHandlerDirectives(handlerMethodType reflect.Type, handlerName string) (map[string]handlesDirective, error) {
	files := make(map[string]*ast.File)
	fset := token.NewFileSet()
	directives := make(map[string]handlesDirective)

	for i := 0; i < handlerMethodType.NumMethod(); i++ {
		method := handlerMethodType.Method(i)
		runtimeFunc := runtime.FuncForPC(method.Func.Pointer())
		if runtimeFunc == nil {
			return nil, fmt.Errorf("web: parse error handler directives for %s.%s: %w", handlerName, method.Name, ErrInvalidErrorHandler)
		}

		filename, _ := runtimeFunc.FileLine(method.Func.Pointer())
		if filename == "" {
			return nil, fmt.Errorf("web: parse error handler directives for %s.%s: %w", handlerName, method.Name, ErrInvalidErrorHandler)
		}

		file, ok := files[filename]
		if !ok {
			parsed, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
			if err != nil {
				return nil, fmt.Errorf("web: parse error handler directives in %s: %w", filename, ErrInvalidErrorHandler)
			}
			file = parsed
			files[filename] = file
		}

		directive, ok, err := parseMethodHandlesDirective(file, handlerName, method.Name)
		if err != nil {
			return nil, err
		}
		if ok {
			directives[method.Name] = directive
		}
	}

	return directives, nil
}

func parseMethodHandlesDirective(file *ast.File, handlerName, methodName string) (handlesDirective, bool, error) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Name.Name != methodName {
			continue
		}
		if !receiverMatches(funcDecl.Recv, handlerName) {
			continue
		}

		return parseHandlesDirectiveComments(handlerName, methodName, funcDecl.Doc)
	}
	return handlesDirective{}, false, nil
}

func parseHandlesDirectiveComments(handlerName, methodName string, comments *ast.CommentGroup) (handlesDirective, bool, error) {
	if comments == nil {
		return handlesDirective{}, false, nil
	}

	var found *handlesDirective
	for _, comment := range comments.List {
		text := comment.Text
		switch {
		case strings.HasPrefix(text, "//helix:handles "):
			directive, err := parseHandlesDirective(text)
			if err != nil {
				return handlesDirective{}, false, fmt.Errorf("web: parse error handler directive %s.%s %q: %w", handlerName, methodName, text, err)
			}
			if found != nil {
				return handlesDirective{}, false, fmt.Errorf("web: parse error handler directive %s.%s %q: %w", handlerName, methodName, text, ErrInvalidErrorHandler)
			}
			found = &directive
		case strings.HasPrefix(text, "// helix:handles") || strings.HasPrefix(text, "//+helix:handles") || strings.HasPrefix(text, "// +helix:handles"):
			return handlesDirective{}, false, fmt.Errorf("web: parse error handler directive %s.%s %q: %w", handlerName, methodName, text, ErrInvalidErrorHandler)
		case strings.HasPrefix(text, "//helix:handles") && !strings.HasPrefix(text, "//helix:handles "):
			return handlesDirective{}, false, fmt.Errorf("web: parse error handler directive %s.%s %q: %w", handlerName, methodName, text, ErrInvalidErrorHandler)
		}
	}
	if found == nil {
		return handlesDirective{}, false, nil
	}
	return *found, true, nil
}

func parseHandlesDirective(text string) (handlesDirective, error) {
	fields := strings.Fields(strings.TrimPrefix(text, "//helix:handles "))
	if len(fields) != 1 || fields[0] == "" {
		return handlesDirective{}, ErrInvalidErrorHandler
	}
	if !isGoIdentifier(fields[0]) {
		return handlesDirective{}, ErrInvalidErrorHandler
	}
	return handlesDirective{errorType: fields[0]}, nil
}

func isGoIdentifier(value string) bool {
	for i, r := range value {
		switch {
		case r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return value != ""
}

func newErrorHandlerInvoker(method reflect.Value, directiveErrorType string) (errorHandlerInvoker, error) {
	methodType := method.Type()
	errorArgIndex, err := validateErrorHandlerSignature(methodType)
	if err != nil {
		return nil, err
	}

	errorArgType := methodType.In(errorArgIndex)
	if canonicalErrorTypeName(errorArgType) != directiveErrorType {
		return nil, fmt.Errorf("web: validate handled type %s: %w", directiveErrorType, ErrInvalidErrorHandler)
	}

	return func(ctx Context, err error) (bool, error) {
		target := reflect.New(errorArgType)
		if !errors.As(err, target.Interface()) {
			return false, nil
		}

		errorValue := target.Elem()

		args := make([]reflect.Value, 0, methodType.NumIn())
		if errorArgIndex == 1 {
			args = append(args, reflect.ValueOf(ctx))
		}
		args = append(args, errorValue)

		results := method.Call(args)
		status := int(results[1].Int())
		if status < 100 || status > 599 {
			return true, writeErrorResponse(ctx, fmt.Errorf("web: error handler returned invalid status %d: %w", status, ErrInvalidErrorHandler))
		}

		ctx.Status(status)
		return true, ctx.JSON(results[0].Interface())
	}, nil
}

func validateErrorHandlerSignature(methodType reflect.Type) (int, error) {
	if methodType.NumOut() != 2 || methodType.Out(1) != intType {
		return 0, fmt.Errorf("web: validate error handler signature: %w", ErrInvalidErrorHandler)
	}

	switch methodType.NumIn() {
	case 1:
		if methodType.In(0).Kind() == reflect.Interface || !methodType.In(0).Implements(errorType) {
			return 0, fmt.Errorf("web: validate error handler error argument: %w", ErrInvalidErrorHandler)
		}
		return 0, nil
	case 2:
		if methodType.In(0) != contextType || methodType.In(1).Kind() == reflect.Interface || !methodType.In(1).Implements(errorType) {
			return 0, fmt.Errorf("web: validate error handler arguments: %w", ErrInvalidErrorHandler)
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("web: validate error handler arguments: %w", ErrInvalidErrorHandler)
	}
}

func canonicalErrorTypeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
