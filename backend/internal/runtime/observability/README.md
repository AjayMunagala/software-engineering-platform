# Runtime Observability

- Phase: 3.5.4
- Contract: `1.0.0` (frozen)
- Engineering acceptance: accepted on 2026-07-27
- Network endpoints: out of scope

This package provides transport-neutral structured lifecycle events and
detached metric snapshots. It owns no HTTP listener, exporter protocol,
database connection, SQL, credential, artifact payload, repository path, or
product data.

## Safety boundary

Events are typed values, not arbitrary field maps. Raw errors are never
accepted. Correlation IDs and safe error kinds are bounded machine
identifiers. Metrics use a fixed name and label vocabulary; repository, scan,
artifact, host, database, user, query, and correlation identifiers are not
metric labels.

## Collection

One collector goroutine takes a bounded snapshot immediately and then at the
configured interval. Collections never overlap. Export uses a bounded context;
export failures are counted and safely logged but do not change persistence
readiness. Collection stops before PostgreSQL pools close.

## Package structure

- `interface.go`: narrow source, sink, and service capabilities
- `implementation.go`: structured logging and collection
- `config.go`: immutable configuration and construction
- `model.go`: immutable event, runtime, pool, and metric models
- `errors.go`: stable redacted errors
- `implementation_test.go`: behavior, security, concurrency, and immutability
- `implementation_benchmark_test.go`: deterministic snapshot benchmark
