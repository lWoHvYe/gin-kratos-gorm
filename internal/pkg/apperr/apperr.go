// Package apperr defines transport-agnostic application errors. Handlers map
// them to HTTP status codes / gRPC status codes in one place, so domain code
// never depends on Gin or gRPC.
package apperr

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Code string

const (
	CodeInvalidArgument    Code = "INVALID_ARGUMENT"
	CodeUnauthenticated    Code = "UNAUTHENTICATED"
	CodePermissionDenied   Code = "PERMISSION_DENIED"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeFailedPrecondition Code = "FAILED_PRECONDITION"
	CodeInternal           Code = "INTERNAL"
	CodeUnavailable        Code = "UNAVAILABLE"
	CodeResourceExhausted  Code = "RESOURCE_EXHAUSTED"
)

// Error is an application error with a stable code and a safe client message.
type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func Invalid(format string, args ...any) *Error {
	return New(CodeInvalidArgument, fmt.Sprintf(format, args...))
}
func Unauthenticated(message string) *Error { return New(CodeUnauthenticated, message) }
func Forbidden(message string) *Error       { return New(CodePermissionDenied, message) }
func NotFound(message string) *Error        { return New(CodeNotFound, message) }
func Conflict(message string) *Error        { return New(CodeConflict, message) }
func FailedPrecondition(message string) *Error {
	return New(CodeFailedPrecondition, message)
}
func Internal(message string, err error) *Error { return Wrap(CodeInternal, message, err) }
func Unavailable(message string, err error) *Error {
	return Wrap(CodeUnavailable, message, err)
}

// From extracts an application error, wrapping unknown errors as INTERNAL.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return Wrap(CodeInternal, "internal error", err)
}

// IsCode reports whether err is an application error with the given code.
func IsCode(err error, code Code) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Code == code
}

// HTTPStatus maps an application error to an HTTP status code.
func HTTPStatus(err error) int {
	switch From(err).Code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeFailedPrecondition:
		return http.StatusUnprocessableEntity
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeResourceExhausted:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// GRPCStatus maps an application error to a gRPC status error.
func GRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	ae := From(err)
	var code codes.Code
	switch ae.Code {
	case CodeInvalidArgument:
		code = codes.InvalidArgument
	case CodeUnauthenticated:
		code = codes.Unauthenticated
	case CodePermissionDenied:
		code = codes.PermissionDenied
	case CodeNotFound:
		code = codes.NotFound
	case CodeConflict:
		code = codes.AlreadyExists
	case CodeFailedPrecondition:
		code = codes.FailedPrecondition
	case CodeUnavailable:
		code = codes.Unavailable
	case CodeResourceExhausted:
		code = codes.ResourceExhausted
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}
