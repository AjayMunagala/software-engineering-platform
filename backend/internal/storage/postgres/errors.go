package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

func failure(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return persistence.NewError(persistence.ErrorNotFound, operation, false, nil)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return persistence.NewError(persistence.ErrorInternal, operation, false, err)
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503":
			return persistence.NewError(persistence.ErrorInvalidDependency, operation, false, nil)
		case "23505":
			return persistence.NewError(persistence.ErrorIdempotencyConflict, operation, false, nil)
		case "23514", "22P02", "22003":
			return persistence.NewError(persistence.ErrorInvalidInput, operation, false, nil)
		case "40001", "40P01", "53300", "57P03":
			return persistence.NewError(persistence.ErrorUnavailable, operation, true, nil)
		case "57014":
			return persistence.NewError(persistence.ErrorTimeout, operation, true, nil)
		case "42501":
			return persistence.NewError(persistence.ErrorAuthorizationDenied, operation, false, nil)
		}
	}
	return persistence.NewError(persistence.ErrorInternal, operation, false, nil)
}

func invalid(operation string) error {
	return persistence.NewError(persistence.ErrorInvalidInput, operation, false, nil)
}

func lifecycle(operation string) error {
	return persistence.NewError(persistence.ErrorLifecycleConflict, operation, false, nil)
}

func integrity(operation string) error {
	return persistence.NewError(persistence.ErrorIntegrityFailure, operation, false, nil)
}
