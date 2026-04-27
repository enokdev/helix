package web

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindingMultipleValidationErrors validates that ALL validation errors are returned, not just the first
func TestBindingMultipleValidationErrors(t *testing.T) {
	type UserRequest struct {
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"required,min=18"`
		Phone string `json:"phone" validate:"required"`
	}

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedFields []string
		expectedCount  int
		isMultiError   bool // Whether response should use multi-error format
	}{
		{
			name:           "single field missing",
			body:           `{"age": 25, "phone": "1234567890"}`,
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"email"},
			expectedCount:  1,
			isMultiError:   false, // Single error uses old format for backward compatibility
		},
		{
			name:           "multiple fields missing",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"email", "age", "phone"},
			expectedCount:  3,
			isMultiError:   true,
		},
		{
			name:           "email invalid + age invalid",
			body:           `{"email": "invalid-email", "age": 10, "phone": "1234567890"}`,
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"email", "age"},
			expectedCount:  2,
			isMultiError:   true,
		},
		{
			name:           "all fields invalid",
			body:           `{"email": "not-email", "age": 5, "phone": ""}`,
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"email", "age", "phone"},
			expectedCount:  3,
			isMultiError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &testContext{body: []byte(tt.body)}
			plan, err := newBindingPlan(reflect.TypeOf(UserRequest{}))
			require.NoError(t, err)

			_, err = plan.bind(ctx)
			require.Error(t, err)

			// Check if it's a RequestError
			var reqErr *RequestError
			require.ErrorAs(t, err, &reqErr, "error should be a RequestError")

			// Check status code
			assert.Equal(t, tt.expectedStatus, reqErr.StatusCode())

			// Check response body structure
			respBody := reqErr.ResponseBody()

			if tt.isMultiError {
				// Multi-field errors use ValidationErrorResponse
				errResp, ok := respBody.(ValidationErrorResponse)
				require.True(t, ok, "response body should be ValidationErrorResponse for multi-field errors")

				// Check error count
				assert.Equal(t, tt.expectedCount, len(errResp.Errors), "should return all validation errors")

				// Check that all expected fields are present
				errFields := make(map[string]bool)
				for _, e := range errResp.Errors {
					errFields[e.Field] = true
				}
				for _, expectedField := range tt.expectedFields {
					assert.True(t, errFields[expectedField], fmt.Sprintf("should return error for field %s", expectedField))
				}
			} else {
				// Single error uses ErrorResponse
				errResp, ok := respBody.(ErrorResponse)
				require.True(t, ok, "response body should be ErrorResponse for single error")

				// Check that the field matches the expected single error
				assert.Equal(t, tt.expectedFields[0], errResp.Error.Field)
			}
		})
	}
}

// TestBindingValidationErrorFormat validates the JSON response format
func TestBindingValidationErrorFormat(t *testing.T) {
	type Request struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	ctx := &testContext{body: []byte(`{"name": "", "email": "invalid"}`)}
	plan, err := newBindingPlan(reflect.TypeOf(Request{}))
	require.NoError(t, err)

	_, err = plan.bind(ctx)
	require.Error(t, err)

	// Check if it's a RequestError
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr, "error should be a RequestError")

	// Check response body structure contains errors with field and msg
	respBody := reqErr.ResponseBody()
	errResp, ok := respBody.(ValidationErrorResponse)
	require.True(t, ok, "response body should be ValidationErrorResponse")
	require.Greater(t, len(errResp.Errors), 0, "should have at least one error")

	// Check that each error has field and msg
	for _, e := range errResp.Errors {
		assert.NotEmpty(t, e.Field, "error should have field")
		assert.NotEmpty(t, e.Msg, "error should have message")
	}
}

// TestBindingValidationErrorOrdering validates errors are in deterministic order
func TestBindingValidationErrorOrdering(t *testing.T) {
	type Request struct {
		FirstField  string `json:"first" validate:"required"`
		SecondField string `json:"second" validate:"required"`
		ThirdField  string `json:"third" validate:"required"`
	}

	ctx := &testContext{body: []byte(`{}`)}
	plan, err := newBindingPlan(reflect.TypeOf(Request{}))
	require.NoError(t, err)

	_, err = plan.bind(ctx)
	require.Error(t, err)

	// Check if it's a RequestError
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr, "error should be a RequestError")

	// Check response body structure
	respBody := reqErr.ResponseBody()
	errResp, ok := respBody.(ValidationErrorResponse)
	require.True(t, ok, "response body should be ValidationErrorResponse")
	require.Equal(t, 3, len(errResp.Errors), "should have 3 errors")

	// Errors should be ordered by field position (first, second, third)
	// In Go reflection, fields are ordered by declaration order
	assert.Equal(t, "first", errResp.Errors[0].Field)
	assert.Equal(t, "second", errResp.Errors[1].Field)
	assert.Equal(t, "third", errResp.Errors[2].Field)
}

// testContext is a minimal Context implementation for testing binding
type testContext struct {
	body []byte
}

func (tc *testContext) Method() string      { return http.MethodPost }
func (tc *testContext) Path() string        { return "/test" }
func (tc *testContext) OriginalURL() string { return "/test" }
func (tc *testContext) Body() []byte        { return tc.body }
func (tc *testContext) Query(string) string { return "" }
func (tc *testContext) Param(string) string { return "" }
func (tc *testContext) Header(key string) string {
	if key == "Content-Type" {
		return "application/json"
	}
	return ""
}
func (tc *testContext) IP() string                { return "127.0.0.1" }
func (tc *testContext) Status(int)                {}
func (tc *testContext) SetHeader(string, string)  {}
func (tc *testContext) Send([]byte) error         { return nil }
func (tc *testContext) JSON(any) error            { return nil }
func (tc *testContext) Context() context.Context  { return context.Background() }
func (tc *testContext) Locals(string, ...any) any { return nil }

// ─── Tests Story 14.2 ───────────────────────────────────────────────────────

// TestBindingEmbeddedStructJSON vérifie que le binding JSON visite les anonymous fields.
func TestBindingEmbeddedStructJSON(t *testing.T) {
	type Base struct {
		BaseField string `json:"base_field"`
	}

	t.Run("champs JSON uniquement dans la struct embarquée", func(t *testing.T) {
		type OnlyEmbeddedReq struct {
			Base
		}
		ctx := &testContext{body: []byte(`{"base_field": "val"}`)}
		plan, err := newBindingPlan(reflect.TypeOf(OnlyEmbeddedReq{}))
		require.NoError(t, err, "newBindingPlan doit réussir pour un struct avec tous les tags json dans l'embed")
		assert.Equal(t, bindingKindJSON, plan.kind)

		val, err := plan.bind(ctx)
		require.NoError(t, err)
		result := val.Interface().(OnlyEmbeddedReq)
		assert.Equal(t, "val", result.BaseField)
	})

	t.Run("champs JSON au niveau top et dans la struct embarquée", func(t *testing.T) {
		type MixedReq struct {
			Base
			Name string `json:"name"`
		}
		ctx := &testContext{body: []byte(`{"name": "test", "base_field": "val"}`)}
		plan, err := newBindingPlan(reflect.TypeOf(MixedReq{}))
		require.NoError(t, err)
		assert.Equal(t, bindingKindJSON, plan.kind)

		val, err := plan.bind(ctx)
		require.NoError(t, err)
		result := val.Interface().(MixedReq)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, "val", result.BaseField)
	})
}

// TestBindingNullBodyRejected vérifie que le body JSON "null" est rejeté avec 400.
func TestBindingNullBodyRejected(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name string
		body string
	}{
		{"raw null", "null"},
		{"null avec espaces", "  null  "},
		{"null avec newline", "\nnull\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &testContext{body: []byte(tt.body)}
			plan, err := newBindingPlan(reflect.TypeOf(Req{}))
			require.NoError(t, err)

			_, err = plan.bind(ctx)
			require.Error(t, err)

			var reqErr *RequestError
			require.ErrorAs(t, err, &reqErr)
			assert.Equal(t, http.StatusBadRequest, reqErr.StatusCode())

			body := reqErr.ResponseBody()
			errResp, ok := body.(ErrorResponse)
			require.True(t, ok)
			assert.Equal(t, codeInvalidJSON, errResp.Error.Code)
		})
	}
}

// TestBindingDisallowUnknownFieldsDefault vérifie que les champs inconnus retournent 400 par défaut.
func TestBindingDisallowUnknownFieldsDefault(t *testing.T) {
	type StrictReq struct {
		Name string `json:"name"`
	}

	ctx := &testContext{body: []byte(`{"name": "test", "unknown_field": "value"}`)}
	plan, err := newBindingPlan(reflect.TypeOf(StrictReq{}))
	require.NoError(t, err)
	assert.False(t, plan.allowUnknown)

	_, err = plan.bind(ctx)
	require.Error(t, err)

	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	assert.Equal(t, http.StatusBadRequest, reqErr.StatusCode())

	body := reqErr.ResponseBody()
	errResp, ok := body.(ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, "unknown_field", errResp.Error.Field)
}

// TestBindingAllowUnknownFieldsOptOut vérifie que helix:"allow-unknown" désactive le rejet des champs inconnus.
func TestBindingAllowUnknownFieldsOptOut(t *testing.T) {
	type LenientReq struct {
		_    struct{} `helix:"allow-unknown"` //nolint:unused
		Name string   `json:"name"`
	}

	ctx := &testContext{body: []byte(`{"name": "test", "unknown_field": "value"}`)}
	plan, err := newBindingPlan(reflect.TypeOf(LenientReq{}))
	require.NoError(t, err)
	assert.True(t, plan.allowUnknown)

	val, err := plan.bind(ctx)
	require.NoError(t, err)

	result := val.Interface().(LenientReq)
	assert.Equal(t, "test", result.Name)
}
