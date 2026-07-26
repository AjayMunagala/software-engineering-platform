package app

import (
	"context"
	"io"
	"os"

	runtimeconfig "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	runtimeobservability "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/observability"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
)

type defaultPostgreSQLOpener struct{ factory runtimepostgres.Factory }

type defaultObservabilityFactory struct {
	output io.Writer
	sink   runtimeobservability.Sink
}

func (opener defaultPostgreSQLOpener) Open(ctx context.Context, loaded runtimeconfig.LoadedConfiguration) (PostgreSQLRuntime, error) {
	return opener.factory.Open(ctx, loaded)
}

func (factory defaultObservabilityFactory) Open(configuration runtimeconfig.RuntimeConfig) (runtimeobservability.Service, error) {
	config, err := runtimeobservability.NewConfig(runtimeobservability.ConfigParams{
		ServiceName:    configuration.Logging().ServiceName(),
		Level:          runtimeobservability.Level(configuration.Logging().Level()),
		Format:         runtimeobservability.Format(configuration.Logging().Format()),
		MetricsEnabled: configuration.Observability().MetricsEnabled(),
		Interval:       configuration.Observability().CollectionInterval(),
	})
	if err != nil {
		return nil, err
	}
	return runtimeobservability.NewService(config, factory.output, factory.sink)
}

// NewStarter creates an injectable lifecycle starter.
func NewStarter(loader runtimeconfig.Loader, opener PostgreSQLOpener) (Starter, error) {
	return NewObservedStarter(loader, opener, nil)
}

// NewObservedStarter creates a lifecycle starter with an optional, narrow
// observability factory. A nil factory preserves deterministic test isolation.
func NewObservedStarter(loader runtimeconfig.Loader, opener PostgreSQLOpener, observer ObservabilityFactory) (Starter, error) {
	if loader == nil || opener == nil {
		return nil, newError(ErrorInvalidInput, "starter", nil)
	}
	return &starter{loader: loader, postgres: opener, observability: observer}, nil
}

// NewDefaultStarter wires the accepted configuration and PostgreSQL runtime
// implementations without starting any network listener.
func NewDefaultStarter() Starter {
	value, _ := NewObservedStarter(
		runtimeconfig.NewLoader(),
		defaultPostgreSQLOpener{factory: runtimepostgres.NewFactory()},
		defaultObservabilityFactory{output: os.Stdout},
	)
	return value
}
