package integration

import (
	"context"
	"errors"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

func serviceFailure(err error, operation, reason string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return repository.NewError(repository.ErrorCanceled, operation, "canceled", false, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return repository.NewError(repository.ErrorTimeout, operation, "timeout", true, context.DeadlineExceeded)
	}
	kind := repository.ErrorInternal
	switch persistence.KindOf(err) {
	case persistence.ErrorNotFound:
		kind = repository.ErrorNotFound
	case persistence.ErrorIdempotencyConflict:
		kind = repository.ErrorIdempotencyConflict
	case persistence.ErrorLifecycleConflict, persistence.ErrorDuplicateArtifact:
		kind = repository.ErrorConflict
	case persistence.ErrorInvalidDependency, persistence.ErrorIntegrityFailure:
		kind = repository.ErrorIntegrityFailure
	case persistence.ErrorPayloadTooLarge:
		kind = repository.ErrorMaterializationFailed
	case persistence.ErrorAuthorizationDenied:
		kind = repository.ErrorNotFound
	case persistence.ErrorTimeout:
		kind = repository.ErrorTimeout
	case persistence.ErrorCanceled:
		kind = repository.ErrorCanceled
	case persistence.ErrorUnavailable:
		kind = repository.ErrorPersistenceUnavailable
	case persistence.ErrorInvalidInput, persistence.ErrorUnsupportedVersion:
		kind = repository.ErrorInternal
	}
	return repository.NewError(kind, operation, reason, persistence.IsRetryable(err), err)
}

func integrity(operation, reason string) error {
	return repository.NewError(repository.ErrorIntegrityFailure, operation, reason, false, nil)
}
