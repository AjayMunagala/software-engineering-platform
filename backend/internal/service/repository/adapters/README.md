# Intelligence & Materialization Adapters

Phase 4.0.5 production adapter for the frozen `repository-go/v1` profile.

It resolves an already-authorized local source, executes fresh released RIE and
Go LIE engines, creates the deterministic artifact dependency manifest, and
serializes approved path-free JSON views once into permission-restricted sealed
spools. The scan coordinator receives only immutable metadata and reopenable
payload streams.

The package does not import Persistence Port, PostgreSQL, runtime
infrastructure, SQL, pgx, HTTP, authentication, UI, or AI packages. It never
clones, fetches, mutates, or writes inside the analyzed repository.

The initial codec contract is `canonical-json/1.0.0`. Exact bytes, SHA-256,
size, ordering, dependency ordinals, source redaction, cancellation, cleanup,
tests, fuzzing, and benchmarks are validated before Phase 4.0.5 acceptance.
