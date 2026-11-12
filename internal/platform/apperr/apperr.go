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
	CodeRateLimited        Code = "RATE_LIMITED"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *Error) Unwrap() error { return e.Err }

func New(code Code, msg string) *Error             { return &Error{Code: code, Message: msg} }
func Wrap(code Code, msg string, err error) *Error { return &Error{Code: code, Message: msg, Err: err} }
func Invalid(format string, args ...any) *Error {
	return New(CodeInvalidArgument, fmt.Sprintf(format, args...))
}
func Unauthenticated(msg string) *Error        { return New(CodeUnauthenticated, msg) }
func Forbidden(msg string) *Error              { return New(CodePermissionDenied, msg) }
func NotFound(msg string) *Error               { return New(CodeNotFound, msg) }
func Conflict(msg string) *Error               { return New(CodeConflict, msg) }
func Failed(msg string) *Error                 { return New(CodeFailedPrecondition, msg) }
func Internal(msg string, err error) *Error    { return Wrap(CodeInternal, msg, err) }
func Unavailable(msg string, err error) *Error { return Wrap(CodeUnavailable, msg, err) }
func RateLimited(msg string) *Error            { return New(CodeRateLimited, msg) }

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
func IsCode(err error, code Code) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Code == code
}
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
	case CodeRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
func GRPC(err error) error {
	if err == nil {
		return nil
	}
	ae := From(err)
	var c codes.Code
	switch ae.Code {
	case CodeInvalidArgument:
		c = codes.InvalidArgument
	case CodeUnauthenticated:
		c = codes.Unauthenticated
	case CodePermissionDenied:
		c = codes.PermissionDenied
	case CodeNotFound:
		c = codes.NotFound
	case CodeConflict:
		c = codes.AlreadyExists
	case CodeFailedPrecondition:
		c = codes.FailedPrecondition
	case CodeUnavailable:
		c = codes.Unavailable
	case CodeRateLimited:
		c = codes.ResourceExhausted
	default:
		c = codes.Internal
	}
	return status.Error(c, ae.Message)
}
