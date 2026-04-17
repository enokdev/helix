package web

import (
	"fmt"
	"net/http"
	"reflect"
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

	// First pass: validate all handler signatures before registering any route,
	// so a bad signature never leaves the server in a partial state.
	type pendingRoute struct {
		method  string
		path    string
		handler HandlerFunc
	}
	routes := make([]pendingRoute, 0, len(routeConventions))
	for _, convention := range routeConventions {
		method := controllerValue.MethodByName(convention.handlerName)
		if !method.IsValid() {
			continue
		}

		handler, err := adaptControllerMethod(method)
		if err != nil {
			return fmt.Errorf("web: register controller %s handler %s: %w", controllerType.Name(), convention.handlerName, err)
		}
		routes = append(routes, pendingRoute{
			method:  convention.method,
			path:    prefix + convention.suffix,
			handler: handler,
		})
	}

	if len(routes) == 0 {
		return fmt.Errorf("web: register controller %s: %w", controllerType.Name(), ErrInvalidController)
	}

	// Second pass: register all routes only after all handlers are validated.
	for _, r := range routes {
		if err := server.RegisterRoute(r.method, r.path, r.handler); err != nil {
			return fmt.Errorf("web: register controller %s route %s %s: %w", controllerType.Name(), r.method, r.path, err)
		}
	}
	return nil
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
