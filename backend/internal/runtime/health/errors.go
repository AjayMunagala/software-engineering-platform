package health

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorInvalidConfig     ErrorCode = "invalid_config"
	ErrorInvalidInput      ErrorCode = "invalid_input"
	ErrorInvalidTransition ErrorCode = "invalid_transition"
)

type Error struct {
	code ErrorCode
	step string
}

func newError(code ErrorCode, step string, _ error) error {
	if code != ErrorInvalidConfig && code != ErrorInvalidInput && code != ErrorInvalidTransition {
		code = ErrorInvalidInput
	}
	if step == "" || len(step) > 64 {
		step = "health"
	}
	return &Error{code: code, step: step}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "runtime-health: invalid_input: health"
	}
	return fmt.Sprintf("runtime-health: %s: %s", failure.code, failure.step)
}

func (failure *Error) Code() ErrorCode {
	if failure == nil {
		return ErrorInvalidInput
	}
	return failure.code
}

func CodeOf(err error) ErrorCode {
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code()
	}
	return ErrorInvalidInput
}
