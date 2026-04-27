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
	kind         bindingKind
	target       reflect.Type
	fields       []fieldBinding
	allowUnknown bool
}

type fieldBinding struct {
	indexPath    []int
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

// hasJSONTags rapporte si t ou l'un de ses anonymous fields contient un tag json.
func hasJSONTags(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if externalTagName(f.Tag.Get("json")) != "" {
			return true
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			if hasJSONTags(f.Type) {
				return true
			}
		}
	}
	return false
}

// hasAllowUnknownTag rapporte si t possède un champ avec le tag helix:"allow-unknown".
func hasAllowUnknownTag(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("helix") == "allow-unknown" {
			return true
		}
	}
	return false
}

// collectQueryFields parcourt récursivement t (y compris les anonymous fields) et
// collecte les champs portant un tag query avec leur chemin d'index multi-niveaux.
func collectQueryFields(t reflect.Type, basePath []int, hasQuery *bool, fields *[]fieldBinding) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		currentPath := make([]int, len(basePath)+1)
		copy(currentPath, basePath)
		currentPath[len(basePath)] = i

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if err := collectQueryFields(field.Type, currentPath, hasQuery, fields); err != nil {
				return err
			}
			continue
		}

		queryName := externalTagName(field.Tag.Get("query"))
		if queryName == "" {
			continue
		}

		*hasQuery = true
		if !isSupportedQueryField(field.Type) {
			return fmt.Errorf("web: adapt handler query field %s: %w", field.Name, ErrUnsupportedHandler)
		}
		maxVal := field.Tag.Get("max")
		if maxVal != "" {
			if !isNumericField(field.Type) {
				return fmt.Errorf("web: adapt handler query field %s max tag: %w", field.Name, ErrUnsupportedHandler)
			}
			if err := validateMaxTagValue(maxVal, field.Type); err != nil {
				return fmt.Errorf("web: adapt handler query field %s max tag value %q: %w", field.Name, maxVal, ErrUnsupportedHandler)
			}
		}
		*fields = append(*fields, fieldBinding{
			indexPath:    currentPath,
			name:         queryName,
			defaultValue: field.Tag.Get("default"),
			maxValue:     maxVal,
		})
	}
	return nil
}

func newBindingPlan(target reflect.Type) (*bindingPlan, error) {
	if target.Kind() != reflect.Struct {
		return nil, fmt.Errorf("web: adapt handler: %w", ErrUnsupportedHandler)
	}

	hasQuery := false
	fields := make([]fieldBinding, 0, target.NumField())
	if err := collectQueryFields(target, nil, &hasQuery, &fields); err != nil {
		return nil, err
	}

	hasJSON := hasJSONTags(target)
	allowUnknown := hasAllowUnknownTag(target)

	switch {
	case hasQuery && hasJSON:
		return nil, fmt.Errorf("web: adapt handler mixed binding tags: %w", ErrUnsupportedHandler)
	case hasQuery:
		return &bindingPlan{kind: bindingKindQuery, target: target, fields: fields}, nil
	case hasJSON:
		return &bindingPlan{kind: bindingKindJSON, target: target, allowUnknown: allowUnknown}, nil
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

		target := value.FieldByIndex(field.indexPath)
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
	ct := ctx.Header("Content-Type")
	if ct == "" {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "Content-Type header is required (application/json)")
	}
	if !strings.Contains(strings.ToLower(ct), "application/json") {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "Content-Type must be application/json")
	}
	body := bytes.TrimSpace(ctx.Body())
	if len(body) == 0 {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "request body is required")
	}
	if bytes.Equal(body, []byte("null")) {
		return newRequestError(http.StatusBadRequest, codeInvalidJSON, "", "request body must not be null")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	if !p.allowUnknown {
		decoder.DisallowUnknownFields()
	}
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
		// If multiple validation errors, use multi-field format
		if len(validationErrors) > 1 {
			fieldErrors := make([]FieldError, 0, len(validationErrors))
			for _, ve := range validationErrors {
				fieldErrors = append(fieldErrors, FieldError{
					Field: ve.Field(),
					Msg:   validationErrorMessage(ve),
				})
			}
			return newMultiFieldValidationError(fieldErrors)
		}
		
		// Single validation error - use old format for backward compatibility
		ve := validationErrors[0]
		return newRequestError(http.StatusBadRequest, codeValidationFailed, ve.Field(),
			validationErrorMessage(ve))
	}
	return newRequestError(http.StatusBadRequest, codeValidationFailed, "", "request validation failed")
}

func validationErrorMessage(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s", ve.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", ve.Param())
	default:
		return fmt.Sprintf("invalid value for %s", ve.Tag())
	}
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
		maxInt, err := strconv.ParseInt(maxValue, 10, target.Type().Bits())
		return err != nil || target.Int() > maxInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		maxUint, err := strconv.ParseUint(maxValue, 10, target.Type().Bits())
		return err != nil || target.Uint() > maxUint
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
