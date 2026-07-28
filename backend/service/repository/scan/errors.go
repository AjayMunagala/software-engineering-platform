package scan

import (
	"context"
	"errors"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

var ErrInvalidConfig = errors.New("invalid scan execution configuration")

func mapDependencyError(err error, operation, reason string, fallback repository.ErrorKind) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return repository.NewError(repository.ErrorInternal, operation, "context-ended", false, err)
	}
	kind := repository.KindOf(err)
	if kind != repository.ErrorInternal {
		return err
	}
	return repository.NewError(fallback, operation, reason, false, nil)
}

func contextFailure(ctx context.Context, operation string) error {
	if ctx == nil {
		return repository.NewError(repository.ErrorInvalidInput, operation, "invalid-request", false, nil)
	}
	if err := ctx.Err(); err != nil {
		return repository.NewError(repository.ErrorInternal, operation, "context-ended", false, err)
	}
	return nil
}
