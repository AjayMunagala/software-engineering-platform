package adapters

import (
	"context"
	"errors"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

var (
	ErrArtifactClosed   = errors.New("sealed artifact is closed")
	ErrPayloadTooLarge  = errors.New("artifact exceeds configured payload limit")
	ErrForbiddenContent = errors.New("artifact contains deployment-local source data")
)

func serviceError(kind repository.ErrorKind, operation, reason string, cause error) error {
	if errors.Is(cause, context.Canceled) {
		return repository.NewError(repository.ErrorCanceled, operation, "canceled", false, context.Canceled)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return repository.NewError(repository.ErrorTimeout, operation, "timeout", true, context.DeadlineExceeded)
	}
	return repository.NewError(kind, operation, reason, false, nil)
}

func contextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return repository.NewError(repository.ErrorInvalidInput, operation, "invalid-context", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return serviceError(repository.ErrorInternal, operation, "context-ended", err)
	}
	return nil
}
