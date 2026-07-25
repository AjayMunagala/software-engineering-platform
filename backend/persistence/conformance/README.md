# Persistence Adapter Conformance

This package is the reusable executable contract for every persistence adapter.
It imports only the storage-neutral `persistence` package.

An adapter test supplies an isolated pre-seeded `Factory`. `Run` then verifies:

- every public operation enforces scope isolation;
- cross-scope target lookups use `not_found` and do not disclose existence;
- list operations do not leak records from another scope;
- cross-scope exact reads write no bytes;
- garbage collection cannot remove a referenced payload in another scope;
- published repository, scan, and artifact metadata is visible;
- exact payload bytes and SHA-256 verification agree.

The PostgreSQL adapter will extend this base suite with publication atomicity,
rollback, concurrency, failure injection, retention, recovery, and
large-payload integration tests during Phase 3.4.3.

## Files

- `interface.go` — adapter factory and suite entry point;
- `implementation.go` — executable neutral requirements;
- `config.go` — bounded per-case timeout;
- `model.go` — immutable fixture scenario and operation catalogue;
- `errors.go` — harness configuration/fixture errors;
- `implementation_test.go` — harness self-test using a deterministic test double;
- `implementation_benchmark_test.go` — operation catalogue benchmark.
