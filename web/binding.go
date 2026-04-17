package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

const (
	requestErrorType      = "ValidationError"
	codeValidationFailed  = "VALIDATION_FAILED"
	codeInvalidQueryParam = "INVALID_QUERY_PARAM"
	codeInvalidJSON       = "INVALID_JSON"
)

var requestValidator = newRequestValidator()

type bindingKind int

const (
	bindingKindQuery bindingKind = iota + 1
	bindingKindJSON
)

type controllerArgumentPlan struct {
	useContext bool
	binding    *bindingPlan
}

type bindingPlan struct {
	kind   bindingKind
	target reflect.Type
	fields []fieldBinding
}

type fieldBinding struct {
	index        int
	name         string
	defaultValue string
	maxValue     string
}

func newRequestValidator() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		if name := externalTagName(field.Tag.Get("query")); name != "" {
			return name
		}
		if name := externalTagName(field.Tag.Get("json")); name != "" {
			return name
		}
		return field.Name
	})
	return validate
}

func newControllerArgumentPlan(methodType reflect.Type) (*controllerArgumentPlan, error) {
	plan := &controllerArgumentPlan{}
	argIndex := 0

	if methodType.NumIn() > 0 && methodType.In(0) == contextType {
		plan.useContext = true
		argIndex++
	}
	if remaining := methodType.NumIn() - argIndex; remaining > 1 {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}
	if methodType.NumIn() == argIndex {
		return plan, nil
	}

	binding, err := newBindingPlan(methodType.In(argIndex))
	if err != nil {
		return nil, err
	}
	plan.binding = binding
	return plan, nil
}

func newBindingPlan(target reflect.Type) (*bindingPlan, error) {
	if target.Kind() != reflect.Struct {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}

	hasQuery := false
	hasJSON := false
	fields := make([]fieldBinding, 0, target.NumField())

	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if field.PkgPath != "" {
			continue
		}

		queryName := externalTagName(field.Tag.Get("query"))
		jsonName := externalTagName(field.Tag.Get("json"))
		if queryName != "" {
			hasQuery = true
			if !isSupportedQueryField(field.Type) {
				return nil, fmt.Errorf("web: adapt handler query field %s: %w", field.Name, ErrUnsupportedHandler)
			}
			maxVal := field.Tag.Get("max")
			if maxVal != "" {
				if !isNumericField(field.Type) {
					return nil, fmt.Errorf("web: adapt handler query field %s max tag: %w", field.Name, ErrUnsupportedHandler)
				}
				if err := validateMaxTagValue(maxVal, field.Type); err != nil {
					return nil, fmt.Errorf("web: adapt handler query field %s max tag value %q: %w", field.Name, maxVal, ErrUnsupportedHandler)
				}
			}
			fields = append(fields, fieldBinding{
				index:        i,
				name:         queryName,
				defaultValue: field.Tag.Get("default"),
				maxValue:     maxVal,
			})
		}
		if jsonName != "" {
			hasJSON = true
		}
	}

	switch {
	case hasQuery && hasJSON:
		return nil, fmt.Errorf("web: adapt handler mixed binding tags: %w", ErrUnsupportedHandler)
	case hasQuery:
		return &bindingPlan{kind: bindingKindQuery, target: target, fields: fields}, nil
	case hasJSON:
		return &bindingPlan{kind: bindingKindJSON, target: target}, nil
	default:
		return nil, fmt.Errorf("web: adapt handler untagged struct: %w", ErrUnsupportedHandler)
	}
}

func (p *controllerArgumentPlan) build(ctx Context) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, 2)
	if p.useContext {
		args = append(args, reflect.ValueOf(ctx))
	}
	if p.binding != nil {
		value, err := p.binding.bind(ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	return args, nil
}

func (p *bindingPlan) bind(ctx Context) (reflect.Value, error) {
	value := reflect.New(p.target).Elem()
	switch p.kind {
	case bindingKindQuery:
		if err := p.bindQuery(ctx, value); err != nil {
			return reflect.Value{}, err
		}
	case bindingKindJSON:
		if err := p.bindJSON(ctx, value); err != nil {
			return reflect.Value{}, err
		}
	default:
		return reflect.Value{}, fmt.Errorf("web: bind request: %w", ErrUnsupportedHandler)
	}

	if err := requestValidator.Struct(value.Interface()); err != nil {
		return reflect.Value{}, validationRequestError(err)
	}
	return value, nil
}

func (p *bindingPlan) bindQuery(ctx Context, value reflect.Value) error {
	for _, field := range p.fields {
		raw := ctx.Query(field.name)
		if raw == "" {
			raw = field.defaultValue
		}
		if raw == "" {
			continue
		}

		target := value.Field(field.index)
		if err := setQueryValue(target, raw); err != nil {
			return newRequestError(http.StatusBadRequest, codeInvalidQueryParam, field.name, fmt.Sprintf("%s has invalid value", field.name))
		}
		if field.maxValue != "" && exceedsMax(target, field.maxValue) {
			return newRequestError(http.StatusBadRequest, codeValidationFailed, field.name, fmt.Sprintf("%s must be at most %s", field.name, field.maxValue))
		}
	}
	return nil
}

func (p *bindingPlan) bindJSON(ctx Context, value reflect.Value) error {
	body := bytes.TrimSpace(ctx.Body())
	if len(body) == 0 {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "request body is required")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value.Addr().Interface()); err != nil {
		field := jsonErrorField(err)
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, field, "request body is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "request body must contain a single JSON value")
	}
	return nil
}

func validationRequestError(err error) error {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) && len(validationErrors) > 0 {
		field := validationErrors[0].Field()
		return newRequestError(http.StatusBadRequest, codeValidationFailed, field, fmt.Sprintf("%s failed validation", field))
	}
	return newRequestError(http.StatusBadRequest, codeValidationFailed, "", "request validation failed")
}

func setQueryValue(value reflect.Value, raw string) error {
	target := value
	if value.Kind() == reflect.Ptr {
		target = reflect.New(value.Type().Elem())
		value.Set(target)
		target = target.Elem()
	}

	switch target.Kind() {
	case reflect.String:
		target.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		target.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(raw, 10, target.Type().Bits())
		if err != nil {
			return err
		}
		target.SetUint(parsed)
	default:
		return ErrUnsupportedHandler
	}
	return nil
}

func validateMaxTagValue(maxValue string, fieldType reflect.Type) error {
	ft := fieldType
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	switch ft.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		_, err := strconv.ParseInt(maxValue, 10, ft.Bits())
		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		_, err := strconv.ParseUint(maxValue, 10, ft.Bits())
		return err
	default:
		return fmt.Errorf("unsupported numeric kind %s", ft.Kind())
	}
}

func exceedsMax(value reflect.Value, maxValue string) bool {
	target := value
	if target.Kind() == reflect.Ptr {
		if target.IsNil() {
			return false
		}
		target = target.Elem()
	}

	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		max, err := strconv.ParseInt(maxValue, 10, target.Type().Bits())
		return err != nil || target.Int() > max
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		max, err := strconv.ParseUint(maxValue, 10, target.Type().Bits())
		return err != nil || target.Uint() > max
	default:
		return false
	}
}

func isSupportedQueryField(fieldType reflect.Type) bool {
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	switch fieldType.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isNumericField(fieldType reflect.Type) bool {
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func externalTagName(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return ""
	}
	return name
}

func jsonErrorField(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return typeErr.Field
	}

	const unknownFieldPrefix = "json: unknown field "
	if strings.HasPrefix(err.Error(), unknownFieldPrefix) {
		return strings.Trim(strings.TrimPrefix(err.Error(), unknownFieldPrefix), `"`)
	}
	return ""
}
