# ADR 0014 — pgx v5 PostgreSQL Adapter Driver

- Status: Accepted
- Date: 2026-07-25
- Accepted: 2026-07-26
- Scope: `backend/internal/storage/postgres` only

## Context

Phase 3.4.3 needs a PostgreSQL 18 client that supports explicit transactions,
parameterized SQL, streaming row iteration, native PostgreSQL error codes, and
a database capability that can be supplied by application orchestration
without constructing pools or loading credentials inside the adapter.

## Decision

Use `github.com/jackc/pgx/v5` at v5.10.0. The adapter accepts a minimal
pgx-compatible `Database` capability. A caller may provide `*pgxpool.Pool`, but
pool construction, sizing, health policy, TLS, and environment configuration
remain outside the adapter and outside Phase 3.4.3.

The adapter uses explicit `pgx.Tx` boundaries, PostgreSQL SQLSTATE values for
private error translation, and ordered row iteration for exact payload chunks.
No pgx type appears in the storage-neutral `backend/persistence` package.

## Alternatives

### `database/sql` with a PostgreSQL driver

Rejected for the first adapter because pgx provides the required PostgreSQL
transaction and error semantics directly, while the neutral port already
prevents driver coupling from escaping the adapter.

### Construct `pgxpool.Pool` inside the adapter

Rejected because it would introduce credentials, runtime topology, pool policy,
and environment loading into a package whose responsibility is SQL mapping and
transaction execution only.

### One shared `pgx.Conn`

Rejected because a single connection is not the application concurrency
boundary. The official pgx documentation distinguishes the single connection
from the concurrency-safe pool.

## Consequences

- The adapter has one direct infrastructure dependency on pgx v5.
- Engines and the neutral persistence port remain pgx-free.
- Unit tests can supply a minimal database test double.
- Disposable integration tests may supply `*pgxpool.Pool` without freezing a
  production pooling strategy.
- A future driver change is private to the adapter unless observable neutral
  behavior changes.

## Evidence

- [pgx getting started and pgxpool guidance](https://github.com/jackc/pgx/wiki/Getting-started-with-pgx)
- [pgx package documentation](https://github.com/jackc/pgx/blob/master/doc.go)
- `POSTGRESQL_ADAPTER_VALIDATION_REPORT.md`

## Acceptance

Engineering accepted Phase 3.4.3 on 2026-07-26 after neutral conformance,
PostgreSQL integration, exact-byte and digest verification, atomic publication,
scope isolation, failure recovery, large-payload streaming, regression, vet,
coverage, and Windows/Linux race evidence passed. Phase 3.4.4 is authorized to
perform the final contract review and `1.0.0` freeze; this acceptance does not
authorize runtime configuration, APIs, UI, or credentials.
