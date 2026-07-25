# Storage-Neutral Persistence Port

`persistence` is the Phase 3.4.2 neutral contract between application
orchestration and durable storage adapters.

It contains no SQL, PostgreSQL driver, connection configuration, credentials,
engine imports, artifact imports, or serialization logic.

## Files

- `interface.go` — small capability interfaces and composed `Port`;
- `implementation.go` — validated constructors and detached value helpers;
- `config.go` — accepted storage-neutral validation limits;
- `model.go` — immutable requests, submissions, records, pages, and receipts;
- `errors.go` — stable safe error categories;
- `implementation_test.go` — contract, validation, immutability, and concurrency tests;
- `implementation_benchmark_test.go` — repeatable construction/copy benchmarks.

The reusable adapter suite lives in `persistence/conformance`. The later
PostgreSQL adapter must pass it before the port can be frozen at `1.0.0`.

## Dependency Rule

```text
engines -> immutable artifacts -> orchestration -> persistence -> adapter
```

Engines never import this package. This package never imports engines or
adapters.

## Security Rule

Every public operation carries a validated scope. Adapters must apply scope to
reads, lists, verification, writes, publication, and retention. Cross-scope
target lookups return `not_found` so resource existence is not disclosed.

## Payload Rule

Payload readers/writers are caller-owned and never closed by the port. A stage
must consume and verify the complete input stream. Exact retrieval output is
untrusted until the method returns success.

## Current Status

Candidate API `0.1.0`. PostgreSQL implementation is intentionally absent and
remains gated by Phase 3.4.2 acceptance.
