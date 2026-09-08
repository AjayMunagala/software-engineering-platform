package golang

import (
	"context"
	"errors"
	"github.com/AjayMunagala/software-engineering-platform/backend/die"
)

type Error struct {
	kind  die.ErrorKind
	code  string
	cause error
}

func (e *Error) Error() string             { return "Go dependency analysis: " + e.code }
func (e *Error) Kind() die.ErrorKind       { return e.kind }
func (e *Error) Code() string              { return e.code }
func (e *Error) Unwrap() error             { return e.cause }
func fail(k die.ErrorKind, c string) error { return &Error{kind: k, code: c} }
func check(ctx context.Context) error {
	if ctx == nil {
		return fail(die.ErrorInvalidInput, "nil_context")
	}
	if err := ctx.Err(); err != nil {
		return &Error{kind: die.ErrorCanceled, code: "analysis_canceled", cause: err}
	}
	return nil
}
func safeError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{kind: die.ErrorCanceled, code: "analysis_canceled", cause: err}
	}
	var own *Error
	if errors.As(err, &own) {
		return own
	}
	var core *die.Error
	if errors.As(err, &core) {
		return fail(core.Kind(), "core_rejected_input")
	}
	return fail(die.ErrorInternal, "analysis_failed")
}
