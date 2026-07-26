package app

import (
	"context"
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorInvalidInput ErrorCode = "invalid_input"
	ErrorStartup      ErrorCode = "startup_failed"
	ErrorDraining     ErrorCode = "runtime_draining"
	ErrorCanceled     ErrorCode = "canceled"
	ErrorTimeout      ErrorCode = "timeout"
	ErrorInternal     ErrorCode = "runtime_internal"
)

type Error struct {
	code  ErrorCode
	step  string
	cause error
}

func newError(code ErrorCode, step string, cause error) error {
	if errors.Is(cause, context.Canceled) {
		code = ErrorCanceled
	} else if errors.Is(cause, context.DeadlineExceeded) {
		code = ErrorTimeout
	}
	if code != ErrorInvalidInput && code != ErrorStartup && code != ErrorDraining &&
		code != ErrorCanceled && code != ErrorTimeout && code != ErrorInternal {
		code = ErrorInternal
	}
	if step == "" || len(step) > 64 {
		step = "runtime"
	}
	return &Error{code: code, step: step, cause: cause}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "application-runtime: runtime_internal: runtime"
	}
	return fmt.Sprintf("application-runtime: %s: %s", failure.code, failure.step)
}

func (failure *Error) Unwrap() error {
	if failure == nil {
		return nil
	}
	if errors.Is(failure.cause, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(failure.cause, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func (failure *Error) Code() ErrorCode {
	if failure == nil {
		return ErrorInternal
	}
	return failure.code
}

func CodeOf(err error) ErrorCode {
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code()
	}
	return ErrorInternal
}
