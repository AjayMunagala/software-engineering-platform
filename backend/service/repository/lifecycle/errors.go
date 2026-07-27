package lifecycle

import (
	"context"
	"errors"

	"github.com/AjayMunagala/software-engineering-platform/backend/service/repository"
)

var ErrInvalidConfig = errors.New("invalid repository lifecycle configuration")

func mapDependencyError(err error, operation, reason string, fallback repository.ErrorKind) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return repository.NewError(repository.ErrorInternal, operation, reason, false, err)
	}
	kind := repository.KindOf(err)
	if kind != repository.ErrorInternal {
		return err
	}
	return repository.NewError(fallback, operation, reason, false, nil)
}
