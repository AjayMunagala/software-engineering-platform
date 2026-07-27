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

