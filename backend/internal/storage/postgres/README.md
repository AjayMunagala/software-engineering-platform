# PostgreSQL Persistence Adapter

This package implements the storage-neutral `backend/persistence` port against
the accepted PostgreSQL schema. It owns parameterized SQL, transactions,
ordered fixed 4 MiB payload chunks, exact-byte verification, atomic scan
publication, scoped reads, retention, garbage collection, and safe SQL error
translation.

It does not construct connection pools, read environment variables, load
credentials, run migrations, serialize intelligence artifacts, or import any
RIE/LIE engine package. Callers provide a pgx-compatible database capability.

Validation order is intentional:

1. Run the reusable neutral conformance suite.
2. Run PostgreSQL rollback, chunk, integrity, concurrency, and lifecycle tests.
3. Run backend regression, vet, race tests, and repeatable benchmarks.

Integration tests require an explicitly disposable migrated database through
`POSTGRES_TEST_URL`. No connection string or credential is committed.

