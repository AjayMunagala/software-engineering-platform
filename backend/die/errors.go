package die

import (
	"context"
	"errors"
)

type ErrorKind string

const (
	ErrorInvalidInput  ErrorKind = "invalid_input"
	ErrorIntegrity     ErrorKind = "integrity_failure"
	ErrorLimitExceeded ErrorKind = "limit_exceeded"
	ErrorCanceled      ErrorKind = "canceled"
	ErrorInternal      ErrorKind = "internal"
)

type Error struct {
	kind          ErrorKind
	code, message string
	cause         error
}

func (e *Error) Error() string   { return e.message }
func (e *Error) Unwrap() error   { return e.cause }
func (e *Error) Kind() ErrorKind { return e.kind }
func (e *Error) Code() string    { return e.code }
func newError(kind ErrorKind, code, message string, cause error) error {
	return &Error{kind: kind, code: code, message: message, cause: cause}
}
func ErrorKindOf(err error) ErrorKind {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind()
	}
	return ErrorInternal
}
func contextError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return newError(ErrorCanceled, "analysis_canceled", "dependency analysis canceled", err)
	}
	return err
}
