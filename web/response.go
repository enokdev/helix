package web

import (
	"errors"
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
	ctx.Status(successStatus(method))
	return ctx.JSON(payload)
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
