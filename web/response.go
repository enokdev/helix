package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	internalServerErrorType = "InternalServerError"
	codeInternalError       = "INTERNAL_ERROR"
	internalErrorMessage    = "internal server error"
)

type structuredHTTPError interface {
	error
	StatusCode() int
	ErrorType() string
	ErrorCode() string
	ErrorField() string
}

func writeSuccessResponse(ctx Context, method string, payload any) error {
	status := successStatus(method)

	// Check if handler set a custom status via context
	if customStatus := ctx.Locals("_helix_custom_status"); customStatus != nil {
		var code int
		switch v := customStatus.(type) {
		case int:
			code = v
		case int32:
			code = int(v)
		case int64:
			code = int(v)
		case float64:
			code = int(v)
		}

		if code >= 100 && code < 600 {
			status = code
		}
	}

	ctx.Status(status)
	if err := ctx.JSON(payload); err != nil {
		slog.Default().With("namespace", "web").Error("json serialisation failed",
			"payload_type", fmt.Sprintf("%T", payload),
			"error", err,
		)
		return err
	}
	return nil
}

func writeErrorResponse(ctx Context, err error) error {
	if err == nil {
		return nil
	}

	var structured structuredHTTPError
	if errors.As(err, &structured) {
		status := structured.StatusCode()
		if status < http.StatusContinue || status > 599 {
			status = http.StatusInternalServerError
		}
		ctx.Status(status)
		return ctx.JSON(ErrorResponse{
			Error: ErrorDetail{
				Type:    defaultString(structured.ErrorType(), internalServerErrorType),
				Message: defaultString(structured.Error(), internalErrorMessage),
				Field:   structured.ErrorField(),
				Code:    defaultString(structured.ErrorCode(), codeInternalError),
			},
		})
	}

	ctx.Status(http.StatusInternalServerError)
	return ctx.JSON(ErrorResponse{
		Error: ErrorDetail{
			Type:    internalServerErrorType,
			Message: internalErrorMessage,
			Field:   "",
			Code:    codeInternalError,
		},
	})
}

func successStatus(method string) int {
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
