# Persistence Port and PostgreSQL Adapter 1.0.0

## Release State

Released as `1.0.0` after Phase 3.4.4 engineering acceptance on 2026-07-26.

Annotated release tags:

- `persistence-port/v1.0.0`;
- `postgresql-adapter/v1.0.0`.

## Included Contracts

- storage-neutral `backend/persistence` capability interfaces and immutable
  models;
- reusable `backend/persistence/conformance` adapter contract;
- PostgreSQL 18 reference adapter using pgx v5;
- exact ordered 4 MiB payload chunks;
- 4 GiB operational payload boundary;
- exact-byte SHA-256 verification;
- atomic scan publication;
- repository scope isolation;
- retention and delayed garbage collection;
- stable neutral error categories.

## Excluded

- credentials and environment loading;
- production connection-pool policy;
- runtime migration execution;
- REST/gRPC APIs and UI;
- engine logic or artifact serialization.

## Compatibility

The proposed `1.x` contract preserves existing exported names, method sets,
signatures, error meanings, validation behavior, and zero-value defaults.
Adding methods to an existing interface is breaking. Optional future
capabilities require new standalone interfaces.

Neutral Go values are not a JSON wire format. Exact intelligence-artifact
payloads, codecs, stable IDs, PostgreSQL schema, and the persistence port have
independent versions.

## Evidence

- [Phase 3.4.3 adapter validation](../../Validation/POSTGRESQL_ADAPTER_VALIDATION_REPORT.md)
- [Phase 3.4.4 stabilization report](../../Validation/PERSISTENCE_CONTRACT_STABILIZATION_REPORT.md)
- [Storage-neutral port API](../../API/PERSISTENCE_PORT_V1.md)
- [Changelog](CHANGELOG.md)
- [ADR 0013](../../Decisions/0013-storage-neutral-persistence-port.md)
- [ADR 0014](../../Decisions/0014-pgx-postgresql-adapter.md)

## Promotion Checklist

- [x] neutral conformance;
- [x] PostgreSQL integration;
- [x] large streaming gate;
- [x] regression, shuffle, vet, coverage, and race testing;
- [x] API, compatibility, dependency, security, and documentation reviews;
- [x] engineering acceptance;
- [x] version promotion to `1.0.0`;
- [x] final versioned regression and race run.
