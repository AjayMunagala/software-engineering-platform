# Repository Service Conformance Harness

This package validates observable behavior of candidate Repository Service
implementations without importing engines, persistence, PostgreSQL, runtime,
or transports.

Adapters provide a fresh pre-seeded `Fixture` through `Factory`. The suite
checks:

- repository, scan, and artifact reads;
- exact streaming export;
- repository-scope isolation for reads, lists, and exports;
- idempotent registration and conflict handling;
- archive behavior;
- synchronous fake scan result semantics;
- cancellation behavior;
- context cancellation and idempotent cleanup.

`NewMemoryFactory` is the Phase 4.0.2 thread-safe fake adapter. It is validation
infrastructure, not production repository lifecycle or scan orchestration.

`RunLifecycle` is the additive Phase 4.0.3 lifecycle-only suite. It accepts a
`RepositoryLifecycleService` without requiring scan or artifact capabilities,
so production lifecycle behavior can pass neutral conformance before any
store-specific integration. `NewMemoryLifecycleFactory` exercises that suite
against the existing fake.

`RunScan` is the additive Phase 4.0.4 scan-and-artifact suite. It accepts only
`ScanExecutionService` and `ArtifactQueryService`, so production scan behavior
can pass neutral conformance without implementing repository lifecycle or any
future persistence/runtime adapter.
