package postgres

import (
	"context"
	"errors"
	"fmt"
)

// ErrorCode is a stable, redacted PostgreSQL runtime failure category.
type ErrorCode string

const (
	ErrorInvalidInput        ErrorCode = "invalid_input"
	ErrorTLSMaterial         ErrorCode = "tls_material_invalid"
	ErrorPoolConfiguration   ErrorCode = "pool_configuration_invalid"
	ErrorUnavailable         ErrorCode = "database_unavailable"
	ErrorTimeout             ErrorCode = "timeout"
	ErrorCanceled            ErrorCode = "canceled"
	ErrorSessionInvalid      ErrorCode = "session_invalid"
	ErrorUnsupportedPostgres ErrorCode = "postgresql_version_unsupported"
	ErrorSchemaIncompatible  ErrorCode = "schema_incompatible"
	ErrorPrivilegeDenied     ErrorCode = "privilege_boundary_invalid"
	ErrorAdapterInvalid      ErrorCode = "adapter_invalid"
	ErrorInternal            ErrorCode = "internal"
)

// Error contains safe schema-owned metadata only. Driver text, SQL, paths,
// addresses, identities, and certificate contents are never formatted.
type Error struct {
	code       ErrorCode
	step       string
	capability Capability
	cause      error
}

func newError(code ErrorCode, step string, capability Capability, cause error) error {
	if !code.valid() {
		code = ErrorInternal
	}
	if !safeStep(step) {
		step = "postgres-runtime"
	}
	if !capability.validOrEmpty() {
		capability = ""
	}
	if errors.Is(cause, context.Canceled) {
		code = ErrorCanceled
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		code = ErrorTimeout
	}
	return &Error{code: code, step: step, capability: capability, cause: cause}
}

func (failure *Error) Error() string {
	if failure == nil {
		return "postgres-runtime: internal: postgres-runtime"
	}
	if failure.capability == "" {
		return fmt.Sprintf("postgres-runtime: %s: %s", failure.code, failure.step)
	}
	return fmt.Sprintf("postgres-runtime: %s: %s: %s", failure.code, failure.step, failure.capability)
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

func (failure *Error) Step() string {
	if failure == nil {
		return "postgres-runtime"
	}
	return failure.step
}

func (failure *Error) Capability() Capability {
	if failure == nil {
		return ""
	}
	return failure.capability
}

// CodeOf returns a stable category for any failure.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var failure *Error
	if errors.As(err, &failure) {
		return failure.Code()
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTimeout
	}
	return ErrorInternal
}

func (code ErrorCode) valid() bool {
	switch code {
	case ErrorInvalidInput, ErrorTLSMaterial, ErrorPoolConfiguration,
		ErrorUnavailable, ErrorTimeout, ErrorCanceled, ErrorSessionInvalid,
		ErrorUnsupportedPostgres, ErrorSchemaIncompatible, ErrorPrivilegeDenied,
		ErrorAdapterInvalid, ErrorInternal:
		return true
	default:
		return false
	}
}

func safeStep(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && character != '-' {
			return false
		}
	}
	return true
}

func (capability Capability) validOrEmpty() bool {
	switch capability {
	case "", CapabilityCombined, CapabilityIngest, CapabilityRead, CapabilityRetention:
		return true
	default:
		return false
	}
}
