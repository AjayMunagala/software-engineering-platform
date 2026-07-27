package repository

import (
	"context"
	"errors"
	"fmt"
)

var ErrInvalidConfig = errors.New("invalid repository service configuration")

// ErrorKind is a stable transport- and storage-neutral category.
type ErrorKind string

const (
	ErrorInvalidInput           ErrorKind = "invalid_input"
	ErrorNotFound               ErrorKind = "not_found"
	ErrorConflict               ErrorKind = "conflict"
	ErrorIdempotencyConflict    ErrorKind = "idempotency_conflict"
	ErrorScanAlreadyRunning     ErrorKind = "scan_already_running"
	ErrorOrphanedScan           ErrorKind = "orphaned_scan"
	ErrorSourceUnavailable      ErrorKind = "source_unavailable"
	ErrorAnalysisFailed         ErrorKind = "analysis_failed"
	ErrorMaterializationFailed  ErrorKind = "materialization_failed"
	ErrorIntegrityFailure       ErrorKind = "integrity_failure"
	ErrorPersistenceUnavailable ErrorKind = "persistence_unavailable"
	ErrorTimeout                ErrorKind = "timeout"
	ErrorCanceled               ErrorKind = "canceled"
	ErrorInternal               ErrorKind = "internal"
)

// Error exposes only safe service information. Raw causes never appear in the
// formatted message and only cancellation/deadline causes are unwrapped.
type Error struct {
	kind       ErrorKind
	operation  string
	reasonCode string
	retryable  bool
	cause      error
}

// NewError constructs one safe neutral failure.
func NewError(kind ErrorKind, operation, reasonCode string, retryable bool, cause error) error {
	if !kind.valid() {
		kind = ErrorInternal
	}
	if !safeToken(operation, 128) {
		operation = "repository-service"
	}
	if reasonCode != "" && !safeToken(reasonCode, 128) {
		reasonCode = "internal"
	}
	if errors.Is(cause, context.Canceled) {
		kind, retryable = ErrorCanceled, false
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		kind, retryable = ErrorTimeout, true
	}
	return &Error{kind: kind, operation: operation, reasonCode: reasonCode, retryable: retryable, cause: cause}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "repository-service: internal"
	}
	if failure.reasonCode == "" {
		return fmt.Sprintf("%s: %s", failure.operation, failure.kind)
	}
	return fmt.Sprintf("%s: %s (%s)", failure.operation, failure.kind, failure.reasonCode)
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

func (failure *Error) Kind() ErrorKind {
	if failure == nil {
		return ErrorInternal
	}
	return failure.kind
}
func (failure *Error) Operation() string {
	if failure == nil {
		return "repository-service"
	}
	return failure.operation
}
func (failure *Error) ReasonCode() string {
	if failure == nil {
		return "internal"
	}
	return failure.reasonCode
}
func (failure *Error) Retryable() bool { return failure != nil && failure.retryable }

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

func IsRetryable(err error) bool {
	var failure *Error
	return errors.As(err, &failure) && failure.Retryable()
}

func (kind ErrorKind) valid() bool {
	switch kind {
	case ErrorInvalidInput, ErrorNotFound, ErrorConflict, ErrorIdempotencyConflict,
		ErrorScanAlreadyRunning, ErrorOrphanedScan, ErrorSourceUnavailable,
		ErrorAnalysisFailed, ErrorMaterializationFailed, ErrorIntegrityFailure,
		ErrorPersistenceUnavailable, ErrorTimeout, ErrorCanceled, ErrorInternal:
		return true
	default:
		return false
	}
}
