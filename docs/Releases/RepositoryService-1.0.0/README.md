# Repository Service 1.0.0 Release Candidate

## State

- Candidate implementation: `0.1.0`
- Proposed stable version: `1.0.0`
- Stabilization evidence: complete
- Final engineering acceptance: pending
- Release tags: not created

This package is review evidence, not a release announcement. Promotion occurs
only after the Phase 4.0.8 evidence and open qualification receive explicit
engineering acceptance.

## Included candidate capabilities

- neutral repository registration, query, listing, and archival;
- synchronous deterministic scan execution and cancellation;
- released RIE and Go LIE orchestration through dedicated adapters;
- canonical exact-byte materialization and deterministic manifests;
- atomic Persistence Port publication through the PostgreSQL adapter;
- runtime admission, lifecycle, health, and shutdown integration;
- metadata query, deterministic listing, and streamed exact-byte export;
- scope isolation, idempotency, single-flight, cancellation, reconciliation,
  source redaction, and stable neutral errors.

## Explicit exclusions

- REST, gRPC, GraphQL, HTTP listeners, authentication, and authorization;
- UI, IDE, AI reasoning, patching, or agent orchestration;
- repository clone/fetch, command execution, mutation, builds, or tests;
- asynchronous/distributed workers, queues, schedulers, or leases.

## Evidence

- [Stabilization report](../../Validation/REPOSITORY_SERVICE_STABILIZATION_VALIDATION_REPORT.md)
- [Machine-readable results](../../Validation/REPOSITORY_SERVICE_STABILIZATION_RESULTS.json)
- [API snapshot](API_SNAPSHOT.md)
- [Benchmark summary](BENCHMARK_SUMMARY.md)
- [Supported feature matrix](SUPPORTED_FEATURE_MATRIX.md)
- [Known limitations](KNOWN_LIMITATIONS.md)
- [Changelog](CHANGELOG.md)
- [Release notes](RELEASE_NOTES.md)
- [Operator checklist](../../Operations/REPOSITORY_SERVICE_RELEASE_CHECKLIST.md)

## Proposed tags

- `repository-service/v1.0.0`

The tag may be created only after final acceptance and must point to the final
reviewed version-promotion commit.
