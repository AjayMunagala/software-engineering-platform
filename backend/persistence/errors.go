package persistence

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrInvalidConfig = errors.New("invalid persistence configuration")
)

// ErrorKind is a stable storage-neutral failure category.
type ErrorKind string

const (
	ErrorInvalidInput        ErrorKind = "invalid_input"
	ErrorNotFound            ErrorKind = "not_found"
	ErrorIdempotencyConflict ErrorKind = "idempotency_conflict"
	ErrorLifecycleConflict   ErrorKind = "lifecycle_conflict"
	ErrorDuplicateArtifact   ErrorKind = "duplicate_artifact"
	ErrorInvalidDependency   ErrorKind = "invalid_dependency"
	ErrorUnsupportedVersion  ErrorKind = "unsupported_version"
	ErrorPayloadTooLarge     ErrorKind = "payload_too_large"
	ErrorIntegrityFailure    ErrorKind = "integrity_failure"
	ErrorAuthorizationDenied ErrorKind = "authorization_denied"
	ErrorTimeout             ErrorKind = "timeout"
	ErrorCanceled            ErrorKind = "canceled"
	ErrorUnavailable         ErrorKind = "unavailable"
	ErrorInternal            ErrorKind = "internal"
)

// Error carries a safe public classification. Cause is available to
// errors.Is/errors.As but is intentionally absent from Error's formatted text.
type Error struct {
	kind      ErrorKind
	operation string
	retryable bool
	cause     error
}

// NewError constructs a safe storage-neutral error.
func NewError(kind ErrorKind, operation string, retryable bool, cause error) error {
	if !kind.valid() {
		kind = ErrorInternal
	}
	if !safeOperation(operation) {
		operation = "persistence"
	}
	if errors.Is(cause, context.Canceled) {
		kind, retryable = ErrorCanceled, false
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		kind, retryable = ErrorTimeout, true
	}
	return &Error{kind: kind, operation: operation, retryable: retryable, cause: cause}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "persistence: internal"
	}
	return fmt.Sprintf("%s: %s", failure.operation, failure.kind)
}

// Unwrap preserves cancellation/deadline matching without exposing the cause
// through the safe Error string.
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

// Kind returns the stable public category.
func (failure *Error) Kind() ErrorKind {
	if failure == nil {
		return ErrorInternal
	}
	return failure.kind
}

// Operation returns the safe logical operation name.
func (failure *Error) Operation() string {
	if failure == nil {
		return "persistence"
	}
	return failure.operation
}

// Retryable reports whether retrying with the same idempotency identity may
// succeed.
func (failure *Error) Retryable() bool { return failure != nil && failure.retryable }

// KindOf returns a stable category for any error.
func KindOf(err error) ErrorKind {
	if err == nil {
		return ""
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Kind()
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTimeout
	}
	return ErrorInternal
}

// IsRetryable reports the adapter-provided safe retry decision.
func IsRetryable(err error) bool {
	var failure *Error
	return errors.As(err, &failure) && failure.Retryable()
}

func (kind ErrorKind) valid() bool {
	switch kind {
	case ErrorInvalidInput, ErrorNotFound, ErrorIdempotencyConflict,
		ErrorLifecycleConflict, ErrorDuplicateArtifact, ErrorInvalidDependency,
		ErrorUnsupportedVersion, ErrorPayloadTooLarge, ErrorIntegrityFailure,
		ErrorAuthorizationDenied, ErrorTimeout, ErrorCanceled, ErrorUnavailable,
		ErrorInternal:
		return true
	default:
		return false
	}
}

func safeOperation(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}
