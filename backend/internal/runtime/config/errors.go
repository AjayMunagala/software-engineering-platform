package config

import (
	"context"
	"errors"
	"fmt"
)

// ErrorCode is a stable, safe configuration failure category.
type ErrorCode string

const (
	ErrorInvalidInput      ErrorCode = "invalid_input"
	ErrorUnknownField      ErrorCode = "unknown_field"
	ErrorDuplicateField    ErrorCode = "duplicate_field"
	ErrorInvalidProfile    ErrorCode = "invalid_profile"
	ErrorInvalidValue      ErrorCode = "invalid_value"
	ErrorConflictingValue  ErrorCode = "conflicting_value"
	ErrorUnsupported       ErrorCode = "unsupported_configuration"
	ErrorFileRead          ErrorCode = "configuration_file_read"
	ErrorSecretUnavailable ErrorCode = "secret_unavailable"
	ErrorSecretAmbiguous   ErrorCode = "secret_ambiguous"
	ErrorCanceled          ErrorCode = "canceled"
	ErrorInternal          ErrorCode = "internal"
)

// Error contains only a stable code and a schema-owned field name. Supplied
// values and wrapped error text are intentionally absent from Error().
type Error struct {
	code  ErrorCode
	field string
	cause error
}

func newError(code ErrorCode, field string, cause error) error {
	if !code.valid() {
		code = ErrorInternal
	}
	if !safeField(field) {
		field = "configuration"
	}
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		code = ErrorCanceled
	}
	return &Error{code: code, field: field, cause: cause}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "runtime-config: internal: configuration"
	}
	return fmt.Sprintf("runtime-config: %s: %s", failure.code, failure.field)
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

func (failure *Error) Field() string {
	if failure == nil {
		return "configuration"
	}
	return failure.field
}

// CodeOf returns the stable code for any error.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrorCanceled
	}
	return ErrorInternal
}

func (code ErrorCode) valid() bool {
	switch code {
	case ErrorInvalidInput, ErrorUnknownField, ErrorDuplicateField,
		ErrorInvalidProfile, ErrorInvalidValue, ErrorConflictingValue,
		ErrorUnsupported, ErrorFileRead, ErrorSecretUnavailable,
		ErrorSecretAmbiguous, ErrorCanceled, ErrorInternal:
		return true
	default:
		return false
	}
}

func safeField(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' && character != '.' && character != '-' {
			return false
		}
	}
	return true
}
