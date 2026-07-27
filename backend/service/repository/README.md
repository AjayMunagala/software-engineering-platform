# Repository Service Contract 0.1.0

This package is the transport-neutral Phase 4.0.2 candidate contract.

It contains:

- narrow repository, scan, and artifact capability interfaces;
- immutable constructor-validated request and response values;
- stable redacted service errors;
- bounded neutral configuration;
- immutable analysis-profile registry;
- frozen `repository-service-artifact-id/v1` canonical bytes.

It deliberately contains no implementation of repository lifecycle, scan
orchestration, engines, materialization, persistence, PostgreSQL, SQL, pgx,
runtime pools, HTTP/gRPC, authentication, UI, queues, workers, or AI.

The contract remains at `0.1.0`. Breaking changes are permitted only before the
future `1.0.0` freeze and must be justified by accepted conformance or
integration evidence.

## Source handle warning

`SourceHandle` is sensitive process-local routing data. Its `String` and
`GoString` forms are always redacted. `Reveal` exists only for the future
authorized source-resolver adapter; callers must never persist, return, log,
or label the revealed value.

## Validation

```text
go test ./service/repository/...
go vet ./service/repository/...
go test -shuffle=on -count=5 ./service/repository/...
go test -race ./service/repository/...
go test -run '^$' -bench . -benchmem -count=5 ./service/repository/...
```

For a checksum-verified disposable Go 1.26.2 validation run on Ubuntu:

```text
bash backend/service/repository/tests/bootstrap_linux.sh
```

Phase 4.0.2 evidence is recorded in
`docs/Validation/REPOSITORY_SERVICE_CONTRACT_VALIDATION_REPORT.md`.
