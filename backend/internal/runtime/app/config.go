package app

import (
	"context"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

type defaultPostgreSQLOpener struct{ factory runtimepostgres.Factory }

func (opener defaultPostgreSQLOpener) Open(ctx context.Context, loaded runtimeconfig.LoadedConfiguration) (PostgreSQLRuntime, error) {
	return opener.factory.Open(ctx, loaded)
}

// NewStarter creates an injectable lifecycle starter.
func NewStarter(loader runtimeconfig.Loader, opener PostgreSQLOpener) (Starter, error) {
	if loader == nil || opener == nil {
		return nil, newError(ErrorInvalidInput, "starter", nil)
	}
	return &starter{loader: loader, postgres: opener}, nil
}

// NewDefaultStarter wires the accepted configuration and PostgreSQL runtime
// implementations without starting any network listener.
func NewDefaultStarter() Starter {
	value, _ := NewStarter(
		runtimeconfig.NewLoader(),
		defaultPostgreSQLOpener{factory: runtimepostgres.NewFactory()},
	)
	return value
}
