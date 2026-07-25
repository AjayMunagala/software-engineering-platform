# Phase 3.4.2 Persistence Port and Conformance Validation Report

## Status

- Milestone: Phase 3.4.2 — Neutral Port and Conformance Harness
- Implementation state: accepted on 2026-07-25
- Candidate contract version: `0.1.0`
- Validation date: 2026-07-24
- PostgreSQL adapter: not implemented and not authorized by this evidence
- Database connections/credentials: none used

## Scope

Phase 3.4.2 implemented only:

- `backend/persistence` storage-neutral capability interfaces;
- detached immutable request, submission, record, page, and receipt models;
- constructor validation and defensive copying;
- storage-neutral configuration limits;
- stable safe error kinds and cancellation/deadline behavior;
- `backend/persistence/conformance` reusable adapter test harness;
- tests and repeatable benchmarks.

It did not implement SQL, a PostgreSQL driver, a connection pool, environment
configuration, credentials, migrations at runtime, APIs, UI, authentication,
engine execution, artifact serialization, or business orchestration.

## Implemented Contract

The neutral package exposes the accepted capabilities:

- `RepositoryStore`
- `ScanStore`
- `PayloadStager`
- `PublicationStore`
- `ArtifactReader`
- `IntegrityVerifier`
- `RetentionStore`
- composed `Port`

There is no public transaction manager. Every adapter must own transaction
boundaries inside lifecycle methods.

All request and result collection state is detached. Constructor input slices
and bytes are copied, returned collections are copied, and concurrent read tests
confirm that caller mutation cannot alter an accepted request.

## Integrity and Validation

The candidate validates:

- non-empty safe scope, principal, request, repository, scan, artifact,
  publication, and projection identities;
- bounded versioned names, codecs, media types, attributes, diagnostics,
  statistics, pages, and retention batches;
- the accepted 4 GiB operational payload maximum and 8 GiB schema ceiling;
- one to 256 artifacts per publication;
- no more than 4,096 dependencies per artifact;
- unique artifact IDs and names;
- exact same-manifest dependency IDs, ordinals, source identities, names, and
  versions;
- no dependency duplicates or self-loops;
- projection source-digest agreement, valid JSON, 8 MiB limit, and exact
  SHA-256;
- diagnostic severity, safe relative paths, bounded messages, and locations;
- exact decimal statistics without binary floating point;
- durable record lifecycle/timestamp coherence;
- safe error strings that cannot expose wrapped driver/storage causes.

Physical four-MiB PostgreSQL chunking does not appear in the neutral API.

## Scope-Isolation Conformance Gate

The reusable harness explicitly covers all 18 public operations:

1. register repository;
2. get repository;
3. list repositories;
4. archive repository;
5. begin scan;
6. get scan;
7. list scans;
8. fail scan;
9. cancel scan;
10. stage payload;
11. publish scan;
12. get artifact;
13. list artifacts;
14. export payload;
15. verify payload;
16. mark repository for purge;
17. purge repository batch;
18. garbage-collect payloads.

Cross-scope target operations must return neutral `not_found` so existence is
not disclosed. Cross-scope list operations must omit foreign records. A denied
exact payload export must write zero bytes. Garbage collection under another
scope may not remove a payload still referenced by the primary scope.

The harness also verifies exact byte retrieval, SHA-256/size receipts, and
published repository/scan/artifact metadata. The later PostgreSQL adapter must
run this same suite against an isolated migrated database.

## Validation Results

| Gate | Result |
|---|---|
| Persistence package tests | PASS |
| Conformance harness self-test | PASS |
| Ten repeated package runs | PASS |
| Shuffled execution, three runs | PASS |
| Full backend regression | PASS |
| Full backend `go vet` | PASS |
| Targeted persistence race tests | PASS |
| Full backend race tests | PASS |
| Data races | 0 |
| Forbidden RIE/LIE/storage/SQL/driver dependencies | 0 |
| `git diff --check` | PASS |

Coverage with atomic counters:

| Package | Statement coverage |
|---|---:|
| `backend/persistence` | 86.5% |
| `backend/persistence/conformance` | 88.1% |

Race validation used the installed MSYS2 UCRT64 compiler with
`CGO_ENABLED=1` on Windows.

## Benchmark Environment

- OS: Windows 10 build 26200.8894
- Architecture: amd64
- CPU: 12th Gen Intel Core i5-12450H
- Go: 1.26.2
- Benchmark workers: Go benchmark default, 12 logical execution slots reported
- PostgreSQL: not involved
- Filesystem/cache: not material; benchmarks operate on in-memory neutral values

Five-run results:

| Benchmark | Result range | Memory | Allocations |
|---|---:|---:|---:|
| 100-artifact publication request construction | 24.64–50.56 µs/op | 53,997 B/op | 107 allocs/op |
| 1 MiB projection validation and defensive copy | 4.39–5.33 ms/op; 196.62–238.67 MiB/s | approximately 1,050,000 B/op | 2–3 allocs/op |

The operation-catalogue benchmark is a structural microbenchmark only and is
not a release performance gate. Database throughput, WAL, transaction latency,
and four-MiB payload streaming remain Phase 3.4.3 adapter measurements.

## Dependency Audit

`go list -deps ./persistence/...` contains no:

- `backend/rie`;
- `backend/lie`;
- `backend/internal/storage`;
- `database/sql`;
- pgx/PostgreSQL driver package.

The neutral package imports only Go standard-library packages. The conformance
package imports only the standard library and `backend/persistence`.

## Known Deferrals

- No adapter exists, so transactional staging, atomic publication, SQL error
  mapping, rollback, locking, and physical retention are not yet exercised.
- Idempotent stream consumption and ambiguous-commit recovery require an
  adapter implementation/failure-injection fixture.
- The candidate remains `0.1.0`; naming and ergonomics may be refined through
  Phase 3.4.3 evidence before the `1.0.0` freeze.
- PostgreSQL scope isolation must pass this exact harness before adapter
  acceptance.

These are later milestone responsibilities, not hidden implementation claims.

## Exit-Gate Assessment

The Phase 3.4.2 local gate is satisfied:

- neutral interfaces and detached models are implemented;
- no engine or database coupling exists;
- constructors, errors, immutability, scope-isolation requirements, tests, and
  benchmarks are documented;
- coverage exceeds the project threshold;
- regression, vet, repeatability, and race checks pass.

Engineering accepted this evidence on 2026-07-25. The neutral candidate is
frozen for adapter work and Phase 3.4.3 PostgreSQL adapter implementation is
authorized. The neutral conformance suite must run before PostgreSQL-specific
integration tests. Phase 3.5 and later remain unauthorized.
